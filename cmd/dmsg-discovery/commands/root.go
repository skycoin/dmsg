package commands

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	proxyproto "github.com/pires/go-proxyproto"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmd/dmsg-discovery/internal/api"
	"github.com/skycoin/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/discmetrics"
	"github.com/skycoin/dmsg/dmsghttp"
	"github.com/skycoin/dmsg/metricsutil"
)

const redisPasswordEnvName = "REDIS_PASSWORD"

var (
	sf                cmdutil.ServiceFlags
	addr              string
	redisURL          string
	entryTimeout      time.Duration
	testMode          bool
	enableLoadTesting bool
	pk                cipher.PubKey
	sk                cipher.SecKey
)

func init() {
	sf.Init(RootCmd, "dmsg_disc", "")

	RootCmd.Flags().StringVarP(&addr, "addr", "a", ":9090", "address to bind to")
	RootCmd.Flags().StringVar(&redisURL, "redis", store.DefaultURL, "connections string for a redis store")
	RootCmd.Flags().DurationVar(&entryTimeout, "entry-timeout", store.DefaultTimeout, "discovery entry timeout")
	RootCmd.Flags().BoolVarP(&testMode, "test-mode", "t", false, "in testing mode")
	RootCmd.Flags().BoolVar(&enableLoadTesting, "enable-load-testing", false, "enable load testing")
	RootCmd.Flags().Var(&sk, "sk", "dmsg secret key")
	RootCmd.MarkFlagRequired("sk") //nolint
}

// RootCmd contains commands for dmsg-discovery
var RootCmd = &cobra.Command{
	Use:   "dmsg-discovery",
	Short: "Dmsg Discovery Server for skywire",
	Run: func(_ *cobra.Command, _ []string) {
		if _, err := buildinfo.Get().WriteTo(os.Stdout); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		log := sf.Logger()

		if err := parse(); err != nil {
			log.WithError(err).Fatal("No SK found.")
		}

		metricsutil.ServeHTTPMetrics(log, sf.MetricsAddr)

		db := prepareDB(log)

		var m discmetrics.Metrics
		if sf.MetricsAddr == "" {
			m = discmetrics.NewEmpty()
		} else {
			m = discmetrics.NewVictoriaMetrics()
		}

		// we enable metrics middleware if address is passed
		enableMetrics := sf.MetricsAddr != ""
		a := api.New(log, db, m, testMode, enableLoadTesting, enableMetrics)

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()
		go a.RunBackgroundTasks(ctx, log)
		log.WithField("addr", addr).Info("Serving discovery API...")
		go func() {
			if err := listenAndServe(addr, a); err != nil {
				log.Errorf("ListenAndServe: %v", err)
				cancel()
			}
		}()

		go func() {
			if err := dmsghttp.ListenAndServe(ctx, pk, sk, a, dmsg.DefaultDmsgHTTPPort, log); err != nil {
				log.Errorf("serveHttpOverDmsg: %v", err)
				cancel()
			}
		}()
		<-ctx.Done()
	},
}

func parse() (err error) {
	pk, err = sk.PubKey()
	return err
}

func prepareDB(log logrus.FieldLogger) store.Storer {
	dbConf := &store.Config{
		URL:      redisURL,
		Password: os.Getenv(redisPasswordEnvName),
		Timeout:  entryTimeout,
	}

	db, err := store.NewStore("redis", dbConf)
	if err != nil {
		log.Fatal("Failed to initialize redis store: ", err)
	}

	return db
}

// Execute executes root CLI command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func listenAndServe(addr string, handler http.Handler) error {
	srv := &http.Server{Addr: addr, Handler: handler}
	if addr == "" {
		addr = ":http"
	}
	ln, err := net.Listen("tcp", addr)
	proxyListener := &proxyproto.Listener{Listener: ln}
	defer proxyListener.Close() // nolint:errcheck
	if err != nil {
		return err
	}
	return srv.Serve(proxyListener)
}
