package servermetrics

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// Metrics collects metrics for prometheus.
type Metrics interface {
	RecordSession(delta DeltaType)
	RecordStream(delta DeltaType)
}

// New returns the default implementation of Metrics.
func New() *metricss {
	var m metricss

	m.activeSessionsGauge = metrics.GetOrCreateGauge("active_sessions_count", func() float64 {
		return float64(m.ActiveSessions())
	})

	m.successfulSessions = metrics.GetOrCreateCounter("session_success_total")
	m.failedSessions = metrics.GetOrCreateCounter("session_fail_total")

	m.activeStreamsGauge = metrics.GetOrCreateGauge("active_streams_count", func() float64 {
		return float64(m.ActiveStreams())
	})

	m.successfulStreams = metrics.GetOrCreateCounter("stream_success_total")

	m.failedStreams = metrics.GetOrCreateCounter("stream_fail_total")

	return &m
}

type metricss struct {
	activeSessions int64
	activeStreams  int64

	activeSessionsGauge *metrics.Gauge
	successfulSessions  *metrics.Counter
	failedSessions      *metrics.Counter
	activeStreamsGauge  *metrics.Gauge
	successfulStreams   *metrics.Counter
	failedStreams       *metrics.Counter
}

func (m *metricss) IncActiveSessions() {
	atomic.AddInt64(&m.activeSessions, 1)
}

func (m *metricss) DecActiveSessions() {
	atomic.AddInt64(&m.activeSessions, -1)
}

func (m *metricss) ActiveSessions() int64 {
	return atomic.LoadInt64(&m.activeSessions)
}

func (m *metricss) IncActiveStreams() {
	atomic.AddInt64(&m.activeStreams, 1)
}

func (m *metricss) DecActiveStreams() {
	atomic.AddInt64(&m.activeStreams, -1)
}

func (m *metricss) ActiveStreams() int64 {
	return atomic.LoadInt64(&m.activeStreams)
}

func (m *metricss) RecordSession(delta DeltaType) {
	switch delta {
	case 0:
		m.failedSessions.Inc()
	case 1:
		m.successfulSessions.Inc()
		m.IncActiveSessions()
	case -1:
		m.DecActiveSessions()
	default:
		panic(fmt.Errorf("invalid delta: %d", delta))
	}
}

func (m *metricss) RecordStream(delta DeltaType) {
	switch delta {
	case 0:
		m.failedStreams.Inc()
	case 1:
		m.successfulStreams.Inc()
		m.IncActiveStreams()
	case -1:
		m.DecActiveStreams()
	default:
		panic(fmt.Errorf("invalid delta: %d", delta))
	}
}
