package commands

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
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
	"github.com/skycoin/dmsg/direct"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/discmetrics"
	"github.com/skycoin/dmsg/dmsghttp"
	"github.com/skycoin/dmsg/metricsutil"
)

const redisPasswordEnvName = "REDIS_PASSWORD"

var (
	sf                cmdutil.ServiceFlags
	addr              string
	redisURL          string
	whitelistKeys     string
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
	RootCmd.Flags().StringVar(&whitelistKeys, "whitelist-keys", "", "list of whitelisted keys used network monitor derigstration")
	RootCmd.Flags().DurationVar(&entryTimeout, "entry-timeout", store.DefaultTimeout, "discovery entry timeout")
	RootCmd.Flags().BoolVarP(&testMode, "test-mode", "t", false, "in testing mode")
	RootCmd.Flags().BoolVar(&enableLoadTesting, "enable-load-testing", false, "enable load testing")
	RootCmd.Flags().Var(&sk, "sk", "dmsg secret key")
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

		var err error
		if pk, err = sk.PubKey(); err != nil {
			log.WithError(err).Warn("No SecKey found. Skipping serving on dmsghttp.")
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

		var whitelistPKs []string
		if whitelistKeys != "" {
			whitelistPKs = strings.Split(whitelistKeys, ",")
			for _, v := range whitelistPKs {
				api.WhitelistPKs[v] = true
			}
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()
		go a.RunBackgroundTasks(ctx, log)
		log.WithField("addr", addr).Info("Serving discovery API...")
		go func() {
			if err = listenAndServe(addr, a); err != nil {
				log.Errorf("ListenAndServe: %v", err)
				cancel()
			}
		}()
		if !pk.Null() {
			servers := getServers(ctx, a, log)
			config := &dmsg.Config{
				MinSessions:    0, // listen on all available servers
				UpdateInterval: dmsg.DefaultUpdateInterval,
			}
			var keys cipher.PubKeys
			keys = append(keys, pk)
			dClient := direct.NewClient(direct.GetAllEntries(keys, servers), log)

			dmsgDC, closeDmsgDC, err := direct.StartDmsg(ctx, log, pk, sk, dClient, config)
			if err != nil {
				log.WithError(err).Fatal("failed to start direct dmsg client.")
			}

			defer closeDmsgDC()

			go updateServers(ctx, a, dClient, dmsgDC, log)

			go func() {
				if err = dmsghttp.ListenAndServe(ctx, pk, sk, a, dClient, dmsg.DefaultDmsgHTTPPort, config, dmsgDC, log); err != nil {
					log.Errorf("dmsghttp.ListenAndServe: %v", err)
					cancel()
				}
			}()
		}

		<-ctx.Done()
	},
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

func getServers(ctx context.Context, a *api.API, log logrus.FieldLogger) (servers []*disc.Entry) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		servers, err := a.AllServers(ctx, log)
		if err != nil {
			log.WithError(err).Fatal("Error getting dmsg-servers.")
		}
		if len(servers) > 0 {
			return servers
		}
		log.Warn("No dmsg-servers found, trying again in 1 minute.")
		select {
		case <-ctx.Done():
			return []*disc.Entry{}
		case <-ticker.C:
			getServers(ctx, a, log)
		}
	}
}

func updateServers(ctx context.Context, a *api.API, dClient disc.APIClient, dmsgC *dmsg.Client, log logrus.FieldLogger) {
	ticker := time.NewTicker(time.Second * 10)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			servers, err := a.AllServers(ctx, log)
			if err != nil {
				log.WithError(err).Error("Error getting dmsg-servers.")
				break
			}
			for _, server := range servers {
				dClient.PostEntry(ctx, server)   //nolint
				dmsgC.EnsureSession(ctx, server) //nolint
			}
		}
	}
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
	if err != nil {
		return err
	}
	proxyListener := &proxyproto.Listener{Listener: ln}
	defer proxyListener.Close() // nolint:errcheck
	return srv.Serve(proxyListener)
}
