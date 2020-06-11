package servermetrics

import (
	"fmt"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics collects metrics for prometheus.
type Metrics interface {
	Collectors() []prometheus.Collector
	RecordSession(delta int)
	RecordStream(delta int)
	HandleDisc(next http.Handler) http.HandlerFunc
}

// New returns the default implementation of Metrics.
func New(namespace string) Metrics {
	activeSessions := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "active_sessions_count",
		Help:      "Current number of active sessions.",
	})
	successfulSessions := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "session_success_total",
		Help:      "Total number of successful session dials.",
	})
	failedSessions := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "session_fail_total",
		Help:      "Total number of failed session dials.",
	})
	activeStreams := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "active_streams_count",
		Help:      "Current number of active streams.",
	})
	successfulStreams := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "stream_success_total",
		Help:      "Total number of successful stream dials.",
	})
	failedStreams := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "stream_fail_total",
		Help:      "Total number of failed stream dials.",
	})
	discReqDurations := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: namespace,
		Name:      "disc_request_duration",
		Help:      "Histogram of discovery request durations.",
	}, []string{"code", "method"})

	return &metrics{
		activeSessions:     activeSessions,
		successfulSessions: successfulSessions,
		failedSessions:     failedSessions,
		activeStreams:      activeStreams,
		successfulStreams:  successfulStreams,
		failedStreams:      failedStreams,
		discReqDurations:   discReqDurations,
	}
}

type metrics struct {
	activeSessions     prometheus.Gauge
	successfulSessions prometheus.Counter
	failedSessions     prometheus.Counter

	activeStreams     prometheus.Gauge
	successfulStreams prometheus.Counter
	failedStreams     prometheus.Counter

	discReqDurations prometheus.ObserverVec
}

func (m *metrics) Collectors() []prometheus.Collector {
	return []prometheus.Collector{
		m.activeSessions,
		m.successfulSessions,
		m.failedSessions,
		m.activeStreams,
		m.successfulStreams,
		m.failedStreams,
		m.discReqDurations,
	}
}

func (m *metrics) RecordSession(delta int) {
	switch delta {
	case 0:
		m.failedSessions.Inc()
	case 1:
		m.successfulSessions.Inc()
		m.activeSessions.Inc()
	case -1:
		m.activeSessions.Dec()
	default:
		panic(fmt.Errorf("invalid delta: %d", delta))
	}
}

func (m *metrics) RecordStream(delta int) {
	switch delta {
	case 0:
		m.failedStreams.Inc()
	case 1:
		m.successfulStreams.Inc()
		m.activeStreams.Inc()
	case -1:
		m.activeStreams.Dec()
	default:
		panic(fmt.Errorf("invalid delta: %d", delta))
	}
}

func (m *metrics) HandleDisc(next http.Handler) http.HandlerFunc {
	return promhttp.InstrumentHandlerDuration(m.discReqDurations, next).ServeHTTP
}
