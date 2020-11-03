package models

import (
	"time"

	"github.com/skycoin/dmsg/buildinfo"
)

// HealthcheckResponse is structe of /health endpoint
type HealthcheckResponse struct {
	BuildInfo            *buildinfo.Info `json:"build_info,omitempty"`
	NumberOfClients      int64           `json:"clients,omitempty"`
	NumberOfServers      int64           `json:"servers,omitempty"`
	StartedAt            time.Time       `json:"started_at,omitempty"`
	AvgPackagesPerMinute uint64          `json:"average_packages_per_minute,omitempty"`
	AvgPackagesPerSecond uint64          `json:"average_packages_per_second,omitempty"`
}
