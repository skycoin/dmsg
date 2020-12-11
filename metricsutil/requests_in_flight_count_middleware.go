package metricsutil

import (
	"net/http"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// RequestsInFlightCountMiddleware is a middleware to track current requests-in-flight count.
type RequestsInFlightCountMiddleware struct {
	reqsInFlight      int64
	reqsInFlightGauge *metrics.Gauge
}

// NewRequestsInFlightCountMiddleware constructs `RequestsInFlightCountMiddleware`.
func NewRequestsInFlightCountMiddleware() *RequestsInFlightCountMiddleware {
	m := &RequestsInFlightCountMiddleware{}
	m.reqsInFlightGauge = metrics.NewGauge(`request_ongoing_count`, func() float64 {
		return float64(m.Reqs())
	})

	return m
}

// IncReqs increments requests count.
func (m *RequestsInFlightCountMiddleware) IncReqs() {
	atomic.AddInt64(&m.reqsInFlight, 1)
}

// DecReqs decrements requests count.
func (m *RequestsInFlightCountMiddleware) DecReqs() {
	atomic.AddInt64(&m.reqsInFlight, -1)
}

// Reqs gets requests count.
func (m *RequestsInFlightCountMiddleware) Reqs() int64 {
	return atomic.LoadInt64(&m.reqsInFlight)
}

// Handle adds to the requests count during request serving.
func (m *RequestsInFlightCountMiddleware) Handle(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		m.IncReqs()
		defer m.DecReqs()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}
