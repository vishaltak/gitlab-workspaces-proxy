package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"time"

	"gitlab.com/remote-development/auth-proxy/auth"
	"gitlab.com/remote-development/auth-proxy/config"
	"golang.org/x/sync/errgroup"
)

type Server struct {
	opts           *ServerOptions
	config         *config.Config
	authMiddleware func(http.Handler) http.Handler
}

type ServerOptions struct {
	Port          int
	ConfigChannel <-chan config.Config
}

func New(opts *ServerOptions) *Server {
	return &Server{opts, nil, nil}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.config == nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Printf("No config found")
		return
	}

	for _, upstream := range s.config.Upstreams {
		upstreamHost := fmt.Sprintf("%s:%d", upstream.Host, s.opts.Port)
		if upstreamHost == r.Host {
			u, err := url.Parse(fmt.Sprintf("%s://%s:%d", upstream.BackendProtocol, upstream.Backend, upstream.Port))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				log.Printf("Error in parsing urls %s", err)
				return
			}

			proxy := httputil.NewSingleHostReverseProxy(u)
			proxy.ServeHTTP(w, r)
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Workspace not found"))
}

func (s *Server) Start(ctx context.Context) error {

	srv := &http.Server{
		Handler: s,
	}

	eg, groupCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		<-groupCtx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			return err
		}
		return nil
	})

	eg.Go(func() error {
		for s.config == nil {
			log.Printf("Waiting for config..")
			time.Sleep(time.Second * 5)
		}

		log.Printf("Starting proxy server...")

		if err := http.ListenAndServe(fmt.Sprintf(":%d", s.opts.Port), s.authMiddleware(s)); err != nil {
			return err
		}
		return nil
	})

	eg.Go(func() error {
		for config := range s.opts.ConfigChannel {
			s.config = &config
			s.authMiddleware = auth.NewMiddleware(&config.Auth)
		}

		return nil
	})

	return eg.Wait()
}
