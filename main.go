package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/auth"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/k8s"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/server"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

const (
	workspaceHostTemplateAnnotation = "remotedevelopment.gitlab/workspace-domain-template"
	workspaceIDAnnotation           = "remotedevelopment.gitlab/workspace-id"
)

func main() { //nolint:cyclop
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

	apiFactory := func(accessToken string) gitlab.API {
		return gitlab.NewClient(accessToken, cfg.Auth.Host, gitlab.BearerTokenType)
	}

	upstreamTracker := upstream.NewTracker(logger)

	opts := &server.Options{
		Port:        *port,
		Middleware:  auth.NewMiddleware(logger, &cfg.Auth, upstreamTracker, apiFactory),
		Logger:      logger,
		Tracker:     upstreamTracker,
		MetricsPath: cfg.MetricsPath,
	}

	s := server.New(opts)

	err = k8sClient.GetService(ctx, func(action k8s.InformerAction, svc *v1.Service) {
		workspaceHostTemplate := svc.Annotations[workspaceHostTemplateAnnotation]
		workspaceID := svc.Annotations[workspaceIDAnnotation]

		if workspaceID == "" {
			logger.Info("workspace ID not available",
				zap.String("service_name", svc.Name),
				zap.String("service_namespace", svc.Namespace))
			return
		}

		switch action {
		case k8s.InformerActionAdd:
			addPorts(workspaceID, workspaceHostTemplate, upstreamTracker, svc, logger)
		case k8s.InformerActionUpdate:
			addPorts(workspaceID, workspaceHostTemplate, upstreamTracker, svc, logger)
		case k8s.InformerActionDelete:
			upstreamTracker.Delete(workspaceHostTemplate)
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

func addPorts(workspaceID string, workspaceHostTemplate string, tracker *upstream.Tracker, svc *v1.Service, logger *zap.Logger) {
	for _, port := range svc.Spec.Ports {
		t, err := template.New("workspaceHostTemplate").Parse(workspaceHostTemplate)
		if err != nil {
			logger.Error("Could not parse domain template", zap.Error(err))
			return
		}
		var h bytes.Buffer
		err = t.Execute(&h, map[string]string{"port": strconv.Itoa(port.TargetPort.IntValue())})
		if err != nil {
			logger.Error("Could not parse domain template", zap.Error(err))
			return
		}

		tracker.Add(upstream.HostMapping{
			Host:            h.String(),
			BackendPort:     port.Port,
			Backend:         fmt.Sprintf("%s.%s", svc.ObjectMeta.Name, svc.ObjectMeta.Namespace),
			BackendProtocol: "http",
			WorkspaceID:     workspaceID,
		})
	}
}
