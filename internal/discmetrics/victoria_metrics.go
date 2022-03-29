package discmetrics

import (
	"github.com/skycoin/skywire-utilities/pkg/metricsutil"
)

// VictoriaMetrics implements `Metrics` using `VictoriaMetrics`.
type VictoriaMetrics struct {
	clientsCount *metricsutil.VictoriaMetricsIntGaugeWrapper
	serversCount *metricsutil.VictoriaMetricsIntGaugeWrapper
}

// NewVictoriaMetrics returns the Victoria Metrics implementation of Metrics.
func NewVictoriaMetrics() *VictoriaMetrics {
	return &VictoriaMetrics{
		clientsCount: metricsutil.NewVictoriaMetricsIntGauge("clients_count"),
		serversCount: metricsutil.NewVictoriaMetricsIntGauge("servers_count"),
	}
}

// SetClientsCount implements `Metrics`.
func (m *VictoriaMetrics) SetClientsCount(val int64) {
	m.clientsCount.Set(val)
}

// SetServersCount implements `Metrics`.
func (m *VictoriaMetrics) SetServersCount(val int64) {
	m.serversCount.Set(val)
}
