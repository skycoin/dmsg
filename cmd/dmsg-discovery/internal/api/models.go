package api

import (
	"time"

	"github.com/skycoin/dmsg/buildinfo"
)

// HealthCheckResponse is struct of /health endpoint
type HealthCheckResponse struct {
	BuildInfo            *buildinfo.Info `json:"build_info"`
	NumberOfClients      int64           `json:"clients"`
	NumberOfServers      int64           `json:"servers"`
	StartedAt            time.Time       `json:"started_at,omitempty"`
	AvgPackagesPerMinute uint64          `json:"average_packages_per_minute"`
	AvgPackagesPerSecond uint64          `json:"average_packages_per_second"`
	Error                string          `json:"error,omitempty"`
}
