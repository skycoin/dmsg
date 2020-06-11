package apimetrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics collects metrics for prometheus.
type Metrics interface {
	Collectors() []prometheus.Collector
	Handle(next http.Handler) http.HandlerFunc
}

// New returns the default implementation of Metrics.
func New(namespace string) Metrics {
	reqCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "request_ongoing_count",
		Help:      "Current number of ongoing requests.",
	})
	reqDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "request_duration",
		Help:      "Histogram of request durations.",
	}, []string{"code", "method"})

	return &metrics{
		inFlight:  reqCount,
		durations: reqDurations,
	}
}

type metrics struct {
	inFlight  prometheus.Gauge
	durations prometheus.ObserverVec
}

func (m *metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.inFlight,
		m.durations,
	}
}

func (m *metrics) Handle(next http.Handler) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		h := promhttp.InstrumentHandlerInFlight(m.inFlight, next)
		promhttp.InstrumentHandlerDuration(m.durations, h).ServeHTTP(w, r)
	}
}
