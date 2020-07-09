package commands

import (
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/buildinfo"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/promutil"
	"github.com/SkycoinProject/dmsg/servermetrics"
)

var sf cmdutil.ServiceFlags

func init() {
	sf.Init(rootCmd, "dmsg_srv", "config.json")
}

var rootCmd = &cobra.Command{
	Use:     "dmsg-server",
	Short:   "Dmsg Server for Skywire.",
	PreRunE: func(cmd *cobra.Command, args []string) error { return sf.Check() },
	Run: func(_ *cobra.Command, args []string) {
		if _, err := buildinfo.Get().WriteTo(os.Stdout); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		log := sf.Logger()

		var conf Config
		if err := sf.ParseConfig(os.Args, true, &conf); err != nil {
			log.WithError(err).Fatal()
		}

		m := prepareMetrics(log, sf.Tag, sf.MetricsAddr)

		lis, err := net.Listen("tcp", conf.LocalAddress)
		if err != nil {
			log.Fatalf("Error listening on %s: %v", conf.LocalAddress, err)
		}

		srvConf := dmsg.ServerConfig{
			MaxSessions:    conf.MaxSessions,
			UpdateInterval: conf.UpdateInterval,
		}
		srv := dmsg.NewServer(conf.PubKey, conf.SecKey, disc.NewHTTP(conf.Discovery), &srvConf, m)
		srv.SetLogger(log)

		defer func() { log.WithError(srv.Close()).Info("Closed server.") }()

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
