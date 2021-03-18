package dmsg

import "time"

// Constants.
const (
	// TODO(evanlinjin): Reference the production address on release
	DefaultDiscAddr = "dmsg.discovery.skywire.skycoin.com"

	DefaultMinSessions = 1

	DefaultUpdateInterval = time.Second * 15

	DefaultMaxSessions = 100
)
