package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"gitlab.com/remote-development/auth-proxy/pkg/auth"
	"gitlab.com/remote-development/auth-proxy/pkg/config"
	"gitlab.com/remote-development/auth-proxy/pkg/k8s"
	"gitlab.com/remote-development/auth-proxy/pkg/server"
	"gitlab.com/remote-development/auth-proxy/pkg/upstream"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

const (
	workspaceHostAnnotation = "remotedevelopment.gitlab/workspace-domain"
)

func main() {
	port := flag.Int("port", 9876, "Port on which to listen")
	configFile := flag.String("config", "", "The config file to use")
	kubeconfig := flag.String("kubeconfig", "", "The kubernetes config file")

	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading config file %s", err)
		os.Exit(-1)
	}

	ctx := context.Background()

	k8sClient, err := k8s.New(*kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating kubernetes client %s", err)
		os.Exit(-1)
	}

	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating logger %s", err)
		os.Exit(-1)
	}
	defer func() {
		err = logger.Sync()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error syncing logger %s", err)
			return
		}
	}()

	opts := &server.ServerOptions{
		Port:       *port,
		Middleware: auth.NewMiddleware(logger, &cfg.Auth),
		Logger:     logger,
	}

	s := server.New(opts)

	err = k8sClient.GetService(ctx, func(action k8s.InformerAction, svc *v1.Service) {
		workspaceHost := svc.Annotations[workspaceHostAnnotation]

		switch action {
		case k8s.InformerActionAdd:
			addPorts(workspaceHost, s, svc)
		case k8s.InformerActionUpdate:
			addPorts(workspaceHost, s, svc)
		case k8s.InformerActionDelete:
			s.DeleteUpstream(workspaceHost)
		}
	})
	if err != nil {
		logger.Error("Could not start informer", zap.Error(err))
		return
	}

	err = s.Start(ctx)
	if err != nil {
		logger.Error("Could not start server", zap.Error(err))
	}
}

func addPorts(workspaceHost string, s *server.Server, svc *v1.Service) {
	for i, port := range svc.Spec.Ports {
		h := workspaceHost
		if i != 0 {
			h = fmt.Sprintf("%d-%s", port.Port, workspaceHost)
		}
		s.AddUpstream(upstream.HostMapping{
			Host:            h,
			BackendPort:     port.Port,
			Backend:         fmt.Sprintf("%s.%s", svc.ObjectMeta.Name, svc.ObjectMeta.Namespace),
			BackendProtocol: "http",
		})
	}
}
