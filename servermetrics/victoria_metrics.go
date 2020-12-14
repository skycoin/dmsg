package servermetrics

import (
	"fmt"
	"sync/atomic"

	"github.com/VictoriaMetrics/metrics"
)

// Metrics collects metrics for metrics tracking system.
type Metrics interface {
	RecordSession(delta DeltaType)
	RecordStream(delta DeltaType)
}

type VictoriaMetrics struct {
	activeSessions int64
	activeStreams  int64

	activeSessionsGauge *metrics.Gauge
	successfulSessions  *metrics.Counter
	failedSessions      *metrics.Counter
	activeStreamsGauge  *metrics.Gauge
	successfulStreams   *metrics.Counter
	failedStreams       *metrics.Counter
}

// NewVictoriaMetrics returns the Victoria Metrics implementation of Metrics.
func NewVictoriaMetrics() *VictoriaMetrics {
	var m VictoriaMetrics

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

// IncActiveSessions increments active sessions count.
func (m *VictoriaMetrics) IncActiveSessions() {
	atomic.AddInt64(&m.activeSessions, 1)
}

// DecActiveSessions decrements active sessions count.
func (m *VictoriaMetrics) DecActiveSessions() {
	atomic.AddInt64(&m.activeSessions, -1)
}

// ActiveSessions gets current active sessions count.
func (m *VictoriaMetrics) ActiveSessions() int64 {
	return atomic.LoadInt64(&m.activeSessions)
}

// IncActiveStreams increments active streams count.
func (m *VictoriaMetrics) IncActiveStreams() {
	atomic.AddInt64(&m.activeStreams, 1)
}

// DecActiveStreams decrements active streams count
func (m *VictoriaMetrics) DecActiveStreams() {
	atomic.AddInt64(&m.activeStreams, -1)
}

// ActiveStreams gets current active streams count.
func (m *VictoriaMetrics) ActiveStreams() int64 {
	return atomic.LoadInt64(&m.activeStreams)
}

// RecordSession implements `Metrics`.
func (m *VictoriaMetrics) RecordSession(delta DeltaType) {
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

// RecordStream implements Metrics.
func (m *VictoriaMetrics) RecordStream(delta DeltaType) {
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
