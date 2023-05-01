package logging

import (
	"net/http"

	"go.uber.org/zap"
)

func NewMiddleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			recorder := newResponseRecorder(w)
			next.ServeHTTP(recorder, r)

			logger.Info("HTTP request processed",
				zap.String("path", r.URL.Path),
				zap.String("ip", r.RemoteAddr),
				zap.Int("status", recorder.status),
				zap.String("host", r.Host),
				zap.String("method", r.Method),
			)
		})
	}
}
