package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	opts *Options
}

type Options struct {
	Port       int
	Middleware func(http.Handler) http.Handler
	Logger     *zap.Logger
	Tracker    *upstream.Tracker
}

func New(opts *Options) *Server {
	return &Server{
		opts: opts,
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestHost := strings.Split(r.Host, ":")[0]
	workspaceHostMapping, err := s.opts.Tracker.Get(requestHost)
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
		s.opts.Logger.Info("Error in parsing url", zap.String("url", targetURL.String()), zap.Error(err))
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
		if err := srv.Shutdown(context.Background()); err != nil { //nolint:golint,contextcheck
			return err
		}
		return nil
	})

	eg.Go(func() error {
		s.opts.Logger.Info("Starting proxy server...")

		var handler http.Handler

		if s.opts.Middleware != nil {
			handler = s.opts.Middleware(s)
		} else {
			handler = s
		}

		if err := http.ListenAndServe(fmt.Sprintf(":%d", s.opts.Port), handler); err != nil {
			return err
		}
		return nil
	})

	return eg.Wait()
}
