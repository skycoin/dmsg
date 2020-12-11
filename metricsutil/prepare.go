package metricsutil

import (
	"net/http"

	"github.com/VictoriaMetrics/metrics"
	"github.com/go-chi/chi"
)

// AddMetricsHandle adds a prometheus-format Handle at '/metrics' to the provided serve mux.
func AddMetricsHandle(mux *chi.Mux) {
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		metrics.WritePrometheus(w, true)
	})
}
