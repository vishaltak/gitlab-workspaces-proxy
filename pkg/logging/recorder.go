package logging

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
)

type responseRecorder struct {
	http.ResponseWriter
	status int
}

func newResponseRecorder(w http.ResponseWriter) *responseRecorder {
	return &responseRecorder{w, http.StatusOK}
}

func (r *responseRecorder) WriteHeader(statusCode int) {
	r.ResponseWriter.WriteHeader(statusCode)
	r.status = statusCode
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		// The status will be StatusOK if WriteHeader has not been called yet
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Hijack has to be implemented by the recorder in order to support websocket
// connections needed by the IDE. Without this interface the HTTP request
// cannot be converted to a websocket. The hijack method ensures that we
// can access the underlying raw TCP connection.
func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := r.ResponseWriter.(http.Hijacker)
	if !ok {
		// The underlying connection does not support the Hijacker interface
		// Call the next handler with the original response writer
		return nil, nil, fmt.Errorf("Hijack not supported")
	}

	return hj.Hijack()
}
