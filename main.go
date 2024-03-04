package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strconv"
	"text/template"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/auth"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/k8s"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/logging"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/server"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	v1 "k8s.io/api/core/v1"
)

const (
	workspaceHostTemplateAnnotation = "workspaces.gitlab.com/host-template"
	workspaceIDAnnotation           = "workspaces.gitlab.com/id"
)

func main() { //nolint:cyclop
	configFile := flag.String("config", "", "The config file to use")
	kubeconfig := flag.String("kubeconfig", "", "The kubernetes config file")

	flag.Parse()

	cfg, err := config.LoadConfig(*configFile)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error reading config file %s", err)
		os.Exit(-1)
	}

	ctx := context.Background()

	logConfig := zap.NewProductionConfig()
	logConfig.Level, err = cfg.GetZapLevel()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to read log level %s", err)
		os.Exit(-1)
	}

	logger, err := logConfig.Build()
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create logger %s", err)
		os.Exit(-1)
	}

	defer func() {
		err = logger.Sync()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "failed to sync logger %s", err)
			return
		}
	}()

	k8sClient, err := k8s.New(logger, *kubeconfig)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "failed to create kubernetes client %s", err)
		os.Exit(-1)
	}

	apiFactory := func(accessToken string) gitlab.API {
		return gitlab.NewClient(logger, accessToken, cfg.Auth.Host, gitlab.BearerTokenType)
	}

	upstreamTracker := upstream.NewTracker(logger)
	loggingMiddleware := logging.NewMiddleware(logger)
	authMiddleware := auth.NewMiddleware(logger, &cfg.Auth, upstreamTracker, apiFactory)

	opts := &server.Options{
		HTTPConfig:        cfg.HTTP,
		SSHConfig:         cfg.SSH,
		LoggingMiddleware: loggingMiddleware,
		AuthMiddleware:    authMiddleware,
		Logger:            logger,
		Tracker:           upstreamTracker,
		MetricsPath:       cfg.MetricsPath,
		APIFactory:        apiFactory,
	}

	s := server.New(opts)

	err = k8sClient.GetService(ctx, func(action k8s.InformerAction, svc *v1.Service) {
		workspaceHostTemplate := svc.Annotations[workspaceHostTemplateAnnotation]
		workspaceID := svc.Annotations[workspaceIDAnnotation]

		if workspaceHostTemplate == "" {
			logger.Error("workspace host template annotation not available on kubernetes service",
				logz.ServiceName(svc.Name),
				logz.ServiceNamespace(svc.Namespace),
			)
			return
		}

		if workspaceID == "" {
			logger.Error("workspace id annotation not available on kubernetes service",
				logz.ServiceName(svc.Name),
				logz.ServiceNamespace(svc.Namespace),
			)
			return
		}

		switch action {
		case k8s.InformerActionAdd:
			addPorts(workspaceID, workspaceHostTemplate, upstreamTracker, svc, logger)
		case k8s.InformerActionUpdate:
			addPorts(workspaceID, workspaceHostTemplate, upstreamTracker, svc, logger)
		case k8s.InformerActionDelete:
			upstreamTracker.DeleteByHostname(workspaceHostTemplate)
		}
	})
	if err != nil {
		logger.Error("failed to start informer", logz.Error(err))
		return
	}

	err = s.Start(ctx)
	if err != nil {
		logger.Error("failed to start server", logz.Error(err))
	}
}

func addPorts(workspaceID string, workspaceHostTemplate string, tracker *upstream.Tracker, svc *v1.Service, logger *zap.Logger) {
	for _, port := range svc.Spec.Ports {
		t, err := template.New("workspaceHostTemplate").Parse(workspaceHostTemplate)
		if err != nil {
			logger.Error(
				"failed to parse workspace host template", logz.Error(err),
				logz.WorkspaceHostTemplate(workspaceHostTemplate),
			)
			return
		}
		var h bytes.Buffer
		data := map[string]string{"port": strconv.Itoa(port.TargetPort.IntValue())}
		err = t.Execute(&h, data)
		if err != nil {
			logger.Error(
				"failed to patch values in workspace host template",
				logz.Error(err),
				logz.WorkspaceHostTemplate(workspaceHostTemplate),
				logz.WorkspaceHostTemplateData(data),
			)
			return
		}

		tracker.Add(upstream.HostMapping{
			Hostname:        h.String(),
			BackendPort:     port.Port,
			Backend:         fmt.Sprintf("%s.%s", svc.ObjectMeta.Name, svc.ObjectMeta.Namespace),
			BackendProtocol: "http",
			WorkspaceID:     workspaceID,
			WorkspaceName:   svc.ObjectMeta.Name,
		})
	}
}
