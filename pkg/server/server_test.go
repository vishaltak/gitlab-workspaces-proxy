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

	"github.com/stretchr/testify/require"
	"gitlab.com/remote-development/auth-proxy/pkg/upstream"
	"go.uber.org/zap/zaptest"
)

func TestStartServer(t *testing.T) {

	upstreamSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello World"))
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
			handler := func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					next.ServeHTTP(w, r)
				})
			}

			s := New(&ServerOptions{
				Port:       tr.port,
				Middleware: handler,
				Logger:     logger,
			})

			for _, u := range tr.upstreamsToAdd {
				s.AddUpstream(u)
			}

			for _, u := range tr.upstreamsToRemove {
				s.DeleteUpstream(u)
			}

			go s.Start(ctx)

			res, err := http.Get(fmt.Sprintf("http://localhost:%d", tr.port))
			require.Nil(t, err)

			result, err := io.ReadAll(res.Body)
			require.Nil(t, err)

			require.Equal(t, tr.expectedStatusCode, res.StatusCode)
			require.Equal(t, tr.expectedBody, string(result))
		})
	}
}
