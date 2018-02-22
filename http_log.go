package imageproxy

import (
	"net/http"
	"time"

	"go.uber.org/zap"
)

type statusCapturingWriter struct {
	http.ResponseWriter
	StatusCode int
}

func (scw *statusCapturingWriter) WriteHeader(status int) {
	scw.StatusCode = status
	scw.ResponseWriter.WriteHeader(status)
}

func WithLogging(handler http.Handler, logger *zap.SugaredLogger) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, req *http.Request) {
		scw := statusCapturingWriter{ResponseWriter: writer}
		start := time.Now()

		handler.ServeHTTP(&scw, req)

		duration := time.Since(start).Nanoseconds() / 1000000
		logger.Infow("Responded",
			"req.URL", req.URL, "duration", duration, "status", scw.StatusCode,
		)
	})
}
