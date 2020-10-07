package commands

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/discord"
	"github.com/skycoin/dmsg/promutil"
	"github.com/skycoin/dmsg/servermetrics"
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

		if discordWebhookURL := discord.GetWebhookURLFromEnv(); discordWebhookURL != "" {
			// Workaround for Discord logger hook. Actually, it's Info.
			log.Error(discord.StartLogMessage)
			defer log.Error(discord.StopLogMessage)
		} else {
			log.Info(discord.StartLogMessage)
			defer log.Info(discord.StopLogMessage)
		}

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

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		go func() {
			if err := srv.Serve(lis, conf.PublicAddress); err != nil {
				log.Errorf("Serve: %v", err)
				cancel()
			}
		}()

		<-ctx.Done()
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
