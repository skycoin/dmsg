package commands

import (
	"log"
	"log/syslog"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg/buildinfo"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/api"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/SkycoinProject/dmsg/metrics"
)

const redisPasswordEnvName = "REDIS_PASSWORD"

var (
	addr         string
	metricsAddr  string
	redisURL     string
	entryTimeout time.Duration
	logEnabled   bool
	syslogAddr   string
	tag          string
	testMode     bool
)

var rootCmd = &cobra.Command{
	Use:   "dmsg-discovery",
	Short: "Dmsg Discovery Server for skywire",
	Run: func(_ *cobra.Command, _ []string) {
		if _, err := buildinfo.Get().WriteTo(log.Writer()); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		conf := &store.Config{
			URL:      redisURL,
			Password: os.Getenv(redisPasswordEnvName),
			Timeout:  entryTimeout,
		}

		s, err := store.NewStore("redis", conf)
		if err != nil {
			log.Fatal("Failed to initialize redis store: ", err)
		}

		l, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatal("Failed to open listener: ", err)
		}

		apiLogger := logging.MustGetLogger(tag)
		if !logEnabled {
			apiLogger = nil
		}

		if syslogAddr != "" {
			hook, err := logrussyslog.NewSyslogHook("udp", syslogAddr, syslog.LOG_INFO, tag)
			if err != nil {
				log.Fatalf("Unable to connect to syslog daemon on %v", syslogAddr)
			}
			logging.AddHook(hook)
		}

		logger := api.Logger(apiLogger)
		metrics := api.Metrics(metrics.NewPrometheus("msgdiscovery"))
		testingMode := api.UseTestingMode(testMode)

		api := api.New(s, logger, metrics, testingMode)

		go func() {
			http.Handle("/metrics", promhttp.Handler())
			if err := http.ListenAndServe(metricsAddr, nil); err != nil {
				log.Println("Failed to start metrics API:", err)
			}
		}()

		if apiLogger != nil {
			apiLogger.Infof("Listening on %s", addr)
		}
		log.Fatal(http.Serve(l, api))
	},
}

func init() {
	rootCmd.Flags().StringVarP(&addr, "addr", "a", ":9090", "address to bind to")
	rootCmd.Flags().StringVarP(&metricsAddr, "metrics", "m", ":2121", "address to bind metrics API to")
	rootCmd.Flags().StringVar(&redisURL, "redis", store.DefaultURL, "connections string for a redis store")
	rootCmd.Flags().DurationVar(&entryTimeout, "entry-timeout", store.DefaultTimeout, "discovery entry timeout")
	rootCmd.Flags().BoolVarP(&logEnabled, "log", "l", true, "enable request logging")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVar(&tag, "tag", "dmsg-discovery", "logging tag")
	rootCmd.Flags().BoolVarP(&testMode, "test-mode", "t", false, "in testing mode")
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
