package discmetrics

// Metrics collects metrics for metrics tracking system.
type Metrics interface {
	SetClientsCount(val int64)
	SetServersCount(val int64)
}
