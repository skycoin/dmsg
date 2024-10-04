module github.com/skycoin/dmsg

go 1.21

toolchain go1.21.4

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/VictoriaMetrics/metrics v1.24.0
	github.com/bitfield/script v0.22.1
	github.com/confiant-inc/go-socks5 v0.0.0-20210816151940-c1124825b1d6
	github.com/creack/pty v1.1.18
	github.com/gin-gonic/gin v1.9.1
	github.com/go-chi/chi/v5 v5.0.11
	github.com/go-redis/redis/v8 v8.11.5
	github.com/hashicorp/yamux v0.1.1
	github.com/ivanpirog/coloredcobra v1.0.1
	github.com/json-iterator/go v1.1.12
	github.com/pires/go-proxyproto v0.6.2
	github.com/sirupsen/logrus v1.9.3
	github.com/skycoin/noise v0.0.0-20180327030543-2492fe189ae6
	github.com/skycoin/skycoin v0.28.0
	github.com/skycoin/skywire v1.3.28
	github.com/skycoin/skywire-utilities v1.3.25
	github.com/spf13/cobra v1.7.0
	github.com/stretchr/testify v1.9.0
	golang.org/x/net v0.21.0
	golang.org/x/sys v0.20.0
	golang.org/x/term v0.17.0
	nhooyr.io/websocket v1.8.7
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20230124172434-306776ec8161 // indirect
	github.com/bytedance/sonic v1.10.0 // indirect
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/chenzhuoyu/base64x v0.0.0-20230717121745-296ad89f973d // indirect
	github.com/chenzhuoyu/iasm v0.9.0 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/color v1.15.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.2 // indirect
	github.com/gin-contrib/sse v0.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.15.1 // indirect
	github.com/goccy/go-json v0.10.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/gojq v0.12.13 // indirect
	github.com/itchyny/timefmt-go v0.1.5 // indirect
	github.com/klauspost/compress v1.16.7 // indirect
	github.com/klauspost/cpuid/v2 v2.2.5 // indirect
	github.com/leodido/go-urn v1.2.4 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.0.9 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.2.11 // indirect
	github.com/valyala/fastrand v1.1.0 // indirect
	github.com/valyala/histogram v1.2.0 // indirect
	golang.org/x/arch v0.4.0 // indirect
	golang.org/x/crypto v0.20.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mvdan.cc/sh/v3 v3.7.0 // indirect
)

// Uncomment for tests with alternate branches of 'skywire-utilities'
//replace github.com/skycoin/skywire => ../skywire
//replace github.com/skycoin/skywire => github.com/skycoin/skywire <commit-hash>

// replace github.com/skycoin/skywire-utilities => ../skywire-utilities
// replace github.com/skycoin/skywire-utilities => github.com/skycoin/skywire-utilities
