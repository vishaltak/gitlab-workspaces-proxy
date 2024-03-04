package k8s

import (
	"context"
	"fmt"
	"time"

	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	informerResyncPeriod  = 1 * time.Hour
	WorkspaceServiceLabel = "agent.gitlab.com/id"
)

type InformerAction uint16

const (
	InformerActionAdd InformerAction = iota
	InformerActionUpdate
	InformerActionDelete
)

type Client interface {
	GetService(ctx context.Context, callback func(InformerAction, *v1.Service)) error
}

type KubernetesClient struct {
	clientset *kubernetes.Clientset
	logger    *zap.Logger
}

func New(logger *zap.Logger, kubeconfig string) (*KubernetesClient, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &KubernetesClient{
		logger:    logger,
		clientset: clientset,
	}, nil
}

func (c *KubernetesClient) GetService(ctx context.Context, callback func(InformerAction, *v1.Service)) error {
	factory := informers.NewSharedInformerFactoryWithOptions(c.clientset, informerResyncPeriod, informers.WithTweakListOptions(func(opts *metav1.ListOptions) {
		opts.LabelSelector = WorkspaceServiceLabel
	}))

	svc := factory.Core().V1().Services()
	informer := svc.Informer()

	stopper := make(chan struct{})
	go func() {
		<-ctx.Done()
		stopper <- struct{}{}
	}()

	go factory.Start(stopper)
	if !cache.WaitForCacheSync(stopper, informer.HasSynced) {
		return fmt.Errorf("timed out waiting for caches to sync")
	}

	_, err := informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			svc := obj.(*v1.Service)
			callback(InformerActionAdd, svc)
		},
		UpdateFunc: func(old interface{}, new interface{}) {
			svc := new.(*v1.Service)
			callback(InformerActionUpdate, svc)
		},
		DeleteFunc: func(old interface{}) {
			svc := old.(*v1.Service)
			callback(InformerActionUpdate, svc)
		},
	})

	return err
}
