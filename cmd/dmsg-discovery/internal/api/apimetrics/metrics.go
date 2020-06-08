package apimetrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type Metrics interface {
	Collectors() []prometheus.Collector
	Handle(next http.Handler) http.HandlerFunc
}

func New() Metrics {
	reqCount := prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "request_ongoing_count",
		Help: "Current number of ongoing requests.",
	})

	reqDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "request_duration_seconds",
		Help: "Histogram of request durations.",
	}, []string{"code", "method"})

	return &metrics{
		inFlight:  reqCount,
		durations: reqDurations,
	}
}

type metrics struct {
	inFlight  prometheus.Gauge
	durations *prometheus.HistogramVec
}

func (m *metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.inFlight,
		m.durations,
	}
}

func (m *metrics) Handle(next http.Handler) http.HandlerFunc {
	h := promhttp.InstrumentHandlerInFlight(m.inFlight, next)
	return promhttp.InstrumentHandlerDuration(m.durations, h)
}
