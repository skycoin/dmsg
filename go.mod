module github.com/skycoin/dmsg

go 1.16

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/VictoriaMetrics/metrics v1.18.1
	github.com/creack/pty v1.1.15
	github.com/go-chi/chi/v5 v5.0.8-0.20220103230436-7dbe9a0bd10f
	github.com/go-redis/redis/v8 v8.11.5
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/json-iterator/go v1.1.12
	github.com/klauspost/compress v1.11.0 // indirect
	github.com/pires/go-proxyproto v0.6.2
	github.com/sirupsen/logrus v1.8.1
	github.com/skycoin/noise v0.0.0-20180327030543-2492fe189ae6
	github.com/skycoin/skycoin v0.27.1
	github.com/skycoin/skywire-utilities v0.0.0-20230314131305-bdd8e27cbd46
	github.com/skycoin/yamux v0.0.0-20200803175205-571ceb89da9f
	github.com/spf13/cobra v1.4.0
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519 // indirect
	golang.org/x/net v0.0.0-20211020060615-d418f374d309
	golang.org/x/sys v0.0.0-20220627191245-f75cf1eec38b
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	nhooyr.io/websocket v1.8.2
)

// Uncomment for tests with alternate branches of 'skywire-utilities'
// replace github.com/skycoin/skywire-utilities => ../skywire-utilities
