package logging

import (
	"net/http"

	"gitlab.com/remote-development/gitlab-workspaces-proxy/internal/logz"
	"go.uber.org/zap"
)

func NewMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r)

			logger.Info("processed HTTP request",
				logz.HTTPPath(r.URL.Path),
				logz.HTTPIp(r.RemoteAddr),
				logz.HTTPStatus(recorder.status),
				logz.HTTPHost(r.Host),
				logz.HTTPMethod(r.Method),
				logz.HTTPScheme(r.URL.Scheme),
			)
		})
	}
}
