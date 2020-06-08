package apimetrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
)

func NewEmpty() Metrics {
	return empty{}
}

type empty struct{}

func (empty) Collectors() []prometheus.Collector {
	return nil
}

func (empty) Handle(next http.Handler) http.HandlerFunc {
	return next.ServeHTTP
}
