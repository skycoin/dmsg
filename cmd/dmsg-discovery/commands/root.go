package commands

import (
	"log"
	"log/syslog"
	"net"
	"net/http"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/api"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"

	"github.com/SkycoinProject/dmsg/metrics"
)

var (
	addr        string
	metricsAddr string
	redisURL    string
	logEnabled  bool
	syslogAddr  string
	tag         string
)

var rootCmd = &cobra.Command{
	Use:   "messaging-discovery",
	Short: "Messaging Discovery Server for skywire",
	Run: func(_ *cobra.Command, _ []string) {
		s, err := store.NewStore("redis", redisURL)
		if err != nil {
			log.Fatal("Failed to initialize redis store: ", err)
		}

		l, err := net.Listen("tcp", addr)
		if err != nil {
			log.Fatal("Failed to open listener: ", err)
		}

		logger := logging.MustGetLogger(tag)
		apiLogger := logger
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

		api := api.New(s, apiLogger, metrics.NewPrometheus("msgdiscovery"))

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
	rootCmd.Flags().StringVar(&redisURL, "redis", "redis://localhost:6379", "connections string for a redis store")
	rootCmd.Flags().BoolVarP(&logEnabled, "log", "l", true, "enable request logging")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address. E.g. localhost:514")
	rootCmd.Flags().StringVar(&tag, "tag", "messaging-discovery", "logging tag")
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
