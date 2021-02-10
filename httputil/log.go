package httputil

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/middleware"
	"github.com/sirupsen/logrus"
)

type structuredLogger struct {
	logger logrus.FieldLogger
}

func NewLogMiddleware(logger logrus.FieldLogger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		fn := func(w http.ResponseWriter, r *http.Request) {
			sl := &structuredLogger{logger}
			start := time.Now()
			var requestID string
			if reqID := r.Context().Value(middleware.RequestIDKey); reqID != nil {
				requestID = reqID.(string)
			}
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			newContext := context.WithValue(r.Context(), middleware.LogEntryCtxKey, sl)
			next.ServeHTTP(ww, r.WithContext(newContext))
			latency := time.Since(start)
			fields := logrus.Fields{
				"status":  ww.Status(),
				"took":    latency,
				"remote":  r.RemoteAddr,
				"request": r.RequestURI,
				"method":  r.Method,
			}
			if requestID != "" {
				fields["request_id"] = requestID
			}
			sl.logger.WithFields(fields).Info()

		}
		return http.HandlerFunc(fn)
	}
}

func LogEntrySetField(r *http.Request, key string, value interface{}) {
	if sl, ok := r.Context().Value(middleware.LogEntryCtxKey).(*structuredLogger); ok {
		sl.logger = sl.logger.WithField(key, value)
	}
}
