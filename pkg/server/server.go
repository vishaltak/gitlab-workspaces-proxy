package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/config"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/gitlab"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/sshproxy"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	opts *Options
}

type Options struct {
	HTTPConfig        config.HTTP
	SSHConfig         config.SSH
	LoggingMiddleware func(http.Handler) http.Handler
	AuthMiddleware    func(http.Handler) http.Handler
	Logger            *zap.Logger
	Tracker           *upstream.Tracker
	MetricsPath       string
	APIFactory        gitlab.APIFactory
}

func New(opts *Options) *Server {
	return &Server{
		opts: opts,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestHostName := strings.Split(r.Host, ":")[0]
	workspaceHostMapping, err := s.opts.Tracker.GetByHostname(requestHostName)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		// TODO: Add proper error pages when workspace not found
		// https://gitlab.com/gitlab-org/gitlab/-/issues/407870
		_, _ = w.Write([]byte("Workspace not found"))
		return
	}

	targetURL, err := url.Parse(fmt.Sprintf("%s://%s:%d", workspaceHostMapping.BackendProtocol, workspaceHostMapping.Backend, workspaceHostMapping.BackendPort))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		s.opts.Logger.Info("failed to parse workspace url",
			logz.Error(err),
			logz.WorkspaceURL(targetURL.String()),
		)
		return
	}

	proxy := httputil.NewSingleHostReverseProxy(targetURL)
	proxy.ServeHTTP(w, r)
}

func (s *Server) Start(ctx context.Context) error {
	srv := &http.Server{
		Handler: s,
	}

	eg, groupCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		<-groupCtx.Done()
		// nolint:golint,contextcheck
		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		return nil
	})

	if s.opts.HTTPConfig.Enabled {
		eg.Go(func() error {
			s.opts.Logger.Info("attempting to start HTTP proxy server", logz.Port(s.opts.HTTPConfig.Port))
			mainHandler := s.opts.LoggingMiddleware(s.opts.AuthMiddleware(s))

			mux := http.NewServeMux()
			mux.Handle(s.opts.MetricsPath, promhttp.Handler())
			mux.Handle("/", mainHandler)

			if err := http.ListenAndServe(fmt.Sprintf(":%d", s.opts.HTTPConfig.Port), mux); err != nil {
				return err
			}
			return nil
		})
	}

	if s.opts.SSHConfig.Enabled {
		readyCh := make(chan struct{})
		eg.Go(func() error {
			s.opts.Logger.Info("attempting to start SSH proxy server", logz.Port(s.opts.SSHConfig.Port))
			proxy, err := sshproxy.New(groupCtx, s.opts.Logger, s.opts.Tracker, &s.opts.SSHConfig, s.opts.APIFactory)
			if err != nil {
				return err
			}
			return proxy.Start(groupCtx, fmt.Sprintf("0.0.0.0:%d", s.opts.SSHConfig.Port), readyCh, nil)
		})
		<-readyCh
	}

	if !s.opts.HTTPConfig.Enabled && !s.opts.SSHConfig.Enabled {
		return fmt.Errorf("neither HTTP or SSH server is enabled to serve traffic")
	}

	return eg.Wait()
}
