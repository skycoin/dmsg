package commands

import (
	"bufio"
	"encoding/json"
	"io"
	"log"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/buildinfo"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/promutil"
	"github.com/SkycoinProject/dmsg/servermetrics"
)

var (
	metricsAddr  string
	syslogAddr   string
	tag          string
	cfgFromStdin bool
)

func init() {
	rootCmd.Flags().StringVarP(&metricsAddr, "metrics", "m", "", "address to bind metrics API to")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVar(&tag, "tag", "dmsg_srv", "logging tag")
	rootCmd.Flags().BoolVarP(&cfgFromStdin, "stdin", "i", false, "read configuration from STDIN")
}

var rootCmd = &cobra.Command{
	Use:   "dmsg-server [config.json]",
	Short: "Dmsg Server for skywire",
	Run: func(_ *cobra.Command, args []string) {
		if _, err := buildinfo.Get().WriteTo(log.Writer()); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		configFile := "config.json"
		if len(args) > 0 {
			configFile = args[0]
		}
		conf := parseConfig(configFile)

		logger := logging.MustGetLogger(tag)
		logLevel, err := logging.LevelFromString(conf.LogLevel)
		if err != nil {
			log.Fatal("Failed to parse LogLevel: ", err)
		}
		logging.SetLevel(logLevel)

		if syslogAddr != "" {
			hook, err := logrussyslog.NewSyslogHook("udp", syslogAddr, syslog.LOG_INFO, tag)
			if err != nil {
				logger.Fatalf("Unable to connect to syslog daemon on %v", syslogAddr)
			}
			logging.AddHook(hook)
		}

		m := prepareMetrics(logger, tag, metricsAddr)

		lis, err := net.Listen("tcp", conf.LocalAddress)
		if err != nil {
			logger.Fatalf("Error listening on %s: %v", conf.LocalAddress, err)
		}

		srvConf := dmsg.ServerConfig{
			MaxSessions:    conf.MaxSessions,
			UpdateInterval: conf.UpdateInterval,
		}
		srv := dmsg.NewServer(conf.PubKey, conf.SecKey, disc.NewHTTP(conf.Discovery), &srvConf, m)
		srv.SetLogger(logger)

		defer func() { logger.WithError(srv.Close()).Info("Closed server.") }()

		if err := srv.Serve(lis, conf.PublicAddress); err != nil {
			log.Fatal(err)
		}
	},
}

// Config is a dmsg-server config
type Config struct {
	PubKey         cipher.PubKey `json:"public_key"`
	SecKey         cipher.SecKey `json:"secret_key"`
	Discovery      string        `json:"discovery"`
	LocalAddress   string        `json:"local_address"`
	PublicAddress  string        `json:"public_address"`
	MaxSessions    int           `json:"max_sessions"`
	UpdateInterval time.Duration `json:"update_interval"`
	LogLevel       string        `json:"log_level"`
}

func parseConfig(configFile string) *Config {
	var r io.Reader
	var err error
	if !cfgFromStdin {
		r, err = os.Open(filepath.Clean(configFile))
		if err != nil {
			log.Fatalf("Failed to open config: %s", err)
		}
	} else {
		r = bufio.NewReader(os.Stdin)
	}

	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()

	conf := new(Config)
	if err := dec.Decode(&conf); err != nil {
		log.Fatalf("Failed to decode config from %s: %s", r, err)
	}

	// Ensure defaults.
	if conf.MaxSessions == 0 {
		conf.MaxSessions = 100
	}
	if conf.UpdateInterval == 0 {
		conf.UpdateInterval = dmsg.DefaultUpdateInterval
	}

	return conf
}

func prepareMetrics(log logrus.FieldLogger, tag, addr string) servermetrics.Metrics {
	if addr == "" {
		return servermetrics.NewEmpty()
	}

	m := servermetrics.New(tag)

	mux := http.NewServeMux()
	promutil.AddMetricsHandle(mux, m.Collectors()...)

	log.WithField("addr", addr).Info("Serving metrics...")
	go func() { log.Fatalln(http.ListenAndServe(addr, mux)) }()

	return m
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
