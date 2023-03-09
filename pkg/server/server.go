package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"gitlab.com/remote-development/auth-proxy/pkg/upstream"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	opts         *ServerOptions
	upstreams    map[string]upstream.HostMapping
	upstreamLock sync.RWMutex
}

type ServerOptions struct {
	Port       int
	Middleware func(http.Handler) http.Handler
	Logger     *zap.Logger
}

func New(opts *ServerOptions) *Server {
	return &Server{
		opts:      opts,
		upstreams: make(map[string]upstream.HostMapping),
	}
}

func (s *Server) ShowUpstreams() map[string]upstream.HostMapping {
	return s.upstreams
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.upstreamLock.RLock()
	upstreams := s.upstreams
	s.upstreamLock.RUnlock()

	requestHost := strings.Split(r.Host, ":")[0]

	if upstream, ok := upstreams[requestHost]; ok && upstream.Host == requestHost {
		u, err := url.Parse(fmt.Sprintf("%s://%s:%d", upstream.BackendProtocol, upstream.Backend, upstream.BackendPort))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			s.opts.Logger.Info("Error in parsing url", zap.String("url", u.String()), zap.Error(err))
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(u)
		proxy.ServeHTTP(w, r)
		return
	}

	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("Workspace not found"))
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

func (s *Server) AddUpstream(mapping upstream.HostMapping) {
	s.upstreamLock.Lock()
	defer s.upstreamLock.Unlock()
	s.upstreams[mapping.Host] = mapping
	s.opts.Logger.Info("New upstream added", zap.String("host", mapping.Host), zap.String("backend", mapping.Backend), zap.Int32("backend_port", mapping.BackendPort))
}

func (s *Server) DeleteUpstream(host string) {
	s.upstreamLock.Lock()
	defer s.upstreamLock.Unlock()
	delete(s.upstreams, host)
	s.opts.Logger.Info("Upstream removed", zap.String("host", host))
}
