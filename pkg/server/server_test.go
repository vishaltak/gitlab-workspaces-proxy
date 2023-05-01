package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gitlab.com/remote-development/gitlab-workspaces-proxy/pkg/upstream"
	"go.uber.org/zap/zaptest"
)

func emptyAuthHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func emptyLoggingHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

func TestStartServer(t *testing.T) {
	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("Hello World"))
	}))

	u, err := url.Parse(upstreamSrv.URL)
	require.Nil(t, err)
	port, err := strconv.Atoi(u.Port())
	require.Nil(t, err)

	tt := []struct {
		description        string
		port               int
		expectedStatusCode int
		expectedBody       string
		upstreamsToAdd     []upstream.HostMapping
		upstreamsToRemove  []string
	}{
		{
			description:        "When no upstream is present returns 404",
			port:               8111,
			expectedStatusCode: http.StatusNotFound,
			expectedBody:       "Workspace not found",
			upstreamsToAdd:     []upstream.HostMapping{},
			upstreamsToRemove:  []string{},
		},
		{
			description:        "When one upstream is present routes to upstream",
			port:               8112,
			expectedStatusCode: http.StatusOK,
			expectedBody:       "Hello World",
			upstreamsToAdd: []upstream.HostMapping{
				{
					Host:            "localhost",
					BackendPort:     int32(port),
					Backend:         u.Hostname(),
					BackendProtocol: "http",
				},
			},
			upstreamsToRemove: []string{},
		},
		{
			description:        "When an upstream is deleted does not route to upstream",
			port:               8113,
			expectedStatusCode: http.StatusNotFound,
			expectedBody:       "Workspace not found",
			upstreamsToAdd: []upstream.HostMapping{
				{
					Host:            "localhost",
					BackendPort:     int32(port),
					Backend:         u.Hostname(),
					BackendProtocol: "http",
				},
				{
					Host:            "localhost-two",
					BackendPort:     int32(port),
					Backend:         u.Hostname(),
					BackendProtocol: "http",
				},
			},
			upstreamsToRemove: []string{"localhost"},
		},
	}

	for _, tr := range tt {
		t.Run(tr.description, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			logger := zaptest.NewLogger(t)

			tracker := upstream.NewTracker(logger)
			s := New(&Options{
				Port:              tr.port,
				AuthMiddleware:    emptyAuthHandler,
				LoggingMiddleware: emptyLoggingHandler,
				Logger:            logger,
				Tracker:           tracker,
				MetricsPath:       "/metrics",
			})

			for _, u := range tr.upstreamsToAdd {
				tracker.Add(u)
			}

			for _, u := range tr.upstreamsToRemove {
				tracker.Delete(u)
			}

			go func() {
				err := s.Start(ctx)
				require.Nil(t, err)
			}()
			time.Sleep(2 * time.Second)

			res, err := http.Get(fmt.Sprintf("http://localhost:%d", tr.port))
			require.Nil(t, err)
			defer res.Body.Close()

			result, err := io.ReadAll(res.Body)
			require.Nil(t, err)

			require.Equal(t, tr.expectedStatusCode, res.StatusCode)
			require.Equal(t, tr.expectedBody, string(result))
		})
	}
}

func TestMetricsPath(t *testing.T) {
	port := 8909
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	logger := zaptest.NewLogger(t)
	tracker := upstream.NewTracker(logger)
	s := New(&Options{
		Port:              port,
		AuthMiddleware:    emptyAuthHandler,
		LoggingMiddleware: emptyLoggingHandler,
		Logger:            logger,
		Tracker:           tracker,
		MetricsPath:       "/metrics",
	})

	go func() {
		err := s.Start(ctx)
		require.Nil(t, err)
	}()
	// Added sleep (to fix flaky test) in order to allow the server to start before we make
	// the request
	time.Sleep(2 * time.Second)

	res, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	require.Nil(t, err)
	defer res.Body.Close()

	require.Equal(t, http.StatusOK, res.StatusCode)
}
