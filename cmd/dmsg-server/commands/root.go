package commands

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"time"

	"github.com/go-chi/chi"
	"github.com/go-chi/chi/middleware"
	jsoniter "github.com/json-iterator/go"
	"github.com/pires/go-proxyproto"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmd/dmsg-server/internal/api"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/metricsutil"
	"github.com/skycoin/dmsg/servermetrics"
)

const (
	defaultDiscoveryURL = "https://dmsg.discovery.skywire.skycoin.com"
	defaultPort         = ":8081"
	defaultConfigPath   = "config.json"
)

var (
	sf cmdutil.ServiceFlags
)

func init() {
	sf.Init(rootCmd, "dmsg_srv", defaultConfigPath)
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
		if err := sf.ParseConfig(os.Args, true, &conf, genDefaultConfig); err != nil {
			log.WithError(err).Fatal("parsing config failed, generating default one...")
		}

		var m servermetrics.Metrics
		if sf.MetricsAddr == "" {
			m = servermetrics.NewEmpty()
		} else {
			m = servermetrics.NewVictoriaMetrics()
		}

		metricsutil.ServeHTTPMetrics(log, sf.MetricsAddr)

		r := chi.NewRouter()
		r.Use(middleware.RequestID)
		r.Use(middleware.RealIP)
		r.Use(middleware.Logger)
		r.Use(middleware.Recoverer)

		a := api.New(r, log, m)
		r.Get("/health", a.Health)
		ln, err := net.Listen("tcp", conf.LocalAddress)
		if err != nil {
			log.Fatalf("Error listening on %s: %v", conf.LocalAddress, err)
		}

		lis := &proxyproto.Listener{Listener: ln}
		defer func(lis *proxyproto.Listener) {
			err = lis.Close()
			if err != nil {
				log.Warnf("Error closing listener: %v", err)
			}
		}(lis)

		if err != nil {
			log.Fatalf("Error creating proxy on %s: %v", conf.LocalAddress, err)
		}

		srvConf := dmsg.ServerConfig{
			MaxSessions:    conf.MaxSessions,
			UpdateInterval: conf.UpdateInterval,
		}
		srv := dmsg.NewServer(conf.PubKey, conf.SecKey, disc.NewHTTP(conf.Discovery), &srvConf, m)
		srv.SetLogger(log)

		a.SetDmsgServer(srv)
		defer func() { log.WithError(srv.Close()).Info("Closed server.") }()

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		go a.RunBackgroundTasks(ctx)
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

func genDefaultConfig() (io.ReadCloser, error) {
	pk, sk := cipher.GenerateKeyPair()

	cfg := Config{
		PubKey:        pk,
		SecKey:        sk,
		Discovery:     defaultDiscoveryURL,
		LocalAddress:  fmt.Sprintf("localhost%s", defaultPort),
		PublicAddress: defaultPort,
		MaxSessions:   2048,
		LogLevel:      "info",
	}

	configData, err := jsoniter.MarshalIndent(&cfg, "", " ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default json config: %v", err)
	}

	if err = ioutil.WriteFile(defaultConfigPath, configData, 0600); err != nil {
		return nil, err
	}

	return os.Open(defaultConfigPath)
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
