module github.com/skycoin/dmsg

go 1.16

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/VictoriaMetrics/metrics v1.18.1
	github.com/creack/pty v1.1.15
	github.com/go-chi/chi/v5 v5.0.8-0.20220103230436-7dbe9a0bd10f
	github.com/go-redis/redis v6.15.9+incompatible
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.11.0 // indirect
	github.com/pires/go-proxyproto v0.6.2
	github.com/sirupsen/logrus v1.8.1
	github.com/skycoin/noise v0.0.0-20180327030543-2492fe189ae6
	github.com/skycoin/skycoin v0.27.1
	github.com/skycoin/skywire-utilities v0.0.0-20220311154445-96c4422db953
	github.com/skycoin/yamux v0.0.0-20200803175205-571ceb89da9f
	github.com/spf13/cobra v1.3.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/net v0.0.0-20210813160813-60bc85c4be6d
	golang.org/x/sys v0.0.0-20211216021012-1d35b9e2eb4e
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	nhooyr.io/websocket v1.8.2
)

// Uncomment for tests with alternate branches of 'skywire-utilities'
// replace github.com/skycoin/skywire-utilities => ../skywire-utilities
