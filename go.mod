module github.com/skycoin/dmsg

go 1.16

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/VictoriaMetrics/metrics v1.12.3
	github.com/creack/pty v1.1.15
	github.com/go-chi/chi/v5 v5.0.8-0.20220103230436-7dbe9a0bd10f
	github.com/go-redis/redis v6.15.8+incompatible
	github.com/google/go-cmp v0.5.5 // indirect
	github.com/json-iterator/go v1.1.10
	github.com/klauspost/compress v1.11.0 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/niemeyer/pretty v0.0.0-20200227124842-a10e7caefd8e // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/onsi/ginkgo v1.15.0 // indirect
	github.com/onsi/gomega v1.10.5 // indirect
	github.com/pires/go-proxyproto v0.3.3
	github.com/sirupsen/logrus v1.8.1
	github.com/skycoin/noise v0.0.0-20180327030543-2492fe189ae6
	github.com/skycoin/skycoin v0.27.1
	github.com/skycoin/yamux v0.0.0-20200803175205-571ceb89da9f
	github.com/spf13/cobra v0.0.5
	github.com/stretchr/testify v1.7.0
	golang.org/x/crypto v0.0.0-20210921155107-089bfa567519
	golang.org/x/net v0.0.0-20210226172049-e18ecbb05110
	golang.org/x/sys v0.0.0-20211020174200-9d6173849985
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211 // indirect
	gopkg.in/check.v1 v1.0.0-20200227125254-8fa46927fb4f // indirect
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b // indirect
	nhooyr.io/websocket v1.8.2
)

// Uncomment for tests with alternate branches of 'skywire-utilities'
//replace github.com/skycoin/skywire-utilities => ../skywire-utilities