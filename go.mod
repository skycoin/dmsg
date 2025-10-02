module github.com/skycoin/dmsg

go 1.25

require (
	github.com/ActiveState/termtest/conpty v0.5.0
	github.com/VictoriaMetrics/metrics v1.40.1
	github.com/bitfield/script v0.24.1
	github.com/chen3feng/safecast v0.0.0-20220908170618-81b2ecd47937
	github.com/coder/websocket v1.8.14
	github.com/confiant-inc/go-socks5 v0.0.0-20210816151940-c1124825b1d6
	github.com/creack/pty v1.1.24
	github.com/gin-gonic/gin v1.10.1
	github.com/go-chi/chi/v5 v5.2.3
	github.com/go-redis/redis/v8 v8.11.5
	github.com/hashicorp/yamux v0.1.2
	github.com/ivanpirog/coloredcobra v1.0.1
	github.com/json-iterator/go v1.1.12
	github.com/pires/go-proxyproto v0.8.1
	github.com/sirupsen/logrus v1.9.3
	github.com/skycoin/noise v0.0.0-20180327030543-2492fe189ae6
	github.com/skycoin/skycoin v0.28.1-0.20250914161012-28a0dc172f9e //DO NOT MODIFY v0.28.1-0.20250914161012-28a0dc172f9e
	github.com/skycoin/skywire v1.3.31-rc3.0.20250914170142-e55540041279
	github.com/spf13/cobra v1.10.1
	github.com/stretchr/testify v1.10.0
	golang.org/x/net v0.44.0
	golang.org/x/sys v0.36.0
	golang.org/x/term v0.35.0
)

require github.com/xtaci/smux v1.5.35

require (
	github.com/Azure/go-ansiterm v0.0.0-20250102033503-faa5f7b0171c // indirect
	github.com/bytedance/gopkg v0.1.3 // indirect
	github.com/bytedance/sonic v1.14.1 // indirect
	github.com/bytedance/sonic/loader v0.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.6 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fatih/color v1.18.0 // indirect
	github.com/gabriel-vasile/mimetype v1.4.10 // indirect
	github.com/gin-contrib/sse v1.1.0 // indirect
	github.com/go-playground/locales v0.14.1 // indirect
	github.com/go-playground/universal-translator v0.18.1 // indirect
	github.com/go-playground/validator/v10 v10.27.0 // indirect
	github.com/goccy/go-json v0.10.5 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/itchyny/gojq v0.12.17 // indirect
	github.com/itchyny/timefmt-go v0.1.6 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/leodido/go-urn v1.4.0 // indirect
	github.com/mattn/go-colorable v0.1.14 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/pelletier/go-toml/v2 v2.2.4 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	github.com/ugorji/go/codec v1.3.0 // indirect
	github.com/valyala/fastrand v1.1.0 // indirect
	github.com/valyala/histogram v1.2.0 // indirect
	golang.org/x/arch v0.21.0 // indirect
	golang.org/x/crypto v0.42.0 // indirect
	golang.org/x/text v0.29.0 // indirect
	google.golang.org/protobuf v1.36.9 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	mvdan.cc/sh/v3 v3.12.0 // indirect
)

// Uncomment for tests with alternate branches of 'skywire'
//replace github.com/skycoin/skywire => ../skywire
//replace github.com/skycoin/skywire => github.com/skycoin/skywire <commit-hash>
//replace github.com/skycoin/skywire => github.com/skycoin/skywire v1.3.31-rc3.0.20250914170142-e55540041279
//replace github.com/skycoin/skycoin => github.com/skycoin/skycoin v0.28.1-0.20250823221707-c533551dfabd
