package commands

import (
	"fmt"
	"io"
	"log"
	"log/syslog"
	"net/http"
	"os"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"
	logrussyslog "github.com/sirupsen/logrus/hooks/syslog"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg/buildinfo"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/api"
	"github.com/SkycoinProject/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/SkycoinProject/dmsg/promutil"
)

const redisPasswordEnvName = "REDIS_PASSWORD"

var (
	tag          string
	addr         string
	redisURL     string
	syslogAddr   string
	metricsAddr  string
	logLevel     string
	entryTimeout time.Duration
	testMode     bool
)

func init() {
	rootCmd.Flags().StringVar(&tag, "tag", "dmsg_disc", "logging tag")
	rootCmd.Flags().StringVarP(&addr, "addr", "a", ":9090", "address to bind to")
	rootCmd.Flags().StringVar(&redisURL, "redis", store.DefaultURL, "connections string for a redis store")
	rootCmd.Flags().StringVar(&syslogAddr, "syslog", "", "syslog server address of format <host>:<port>")
	rootCmd.Flags().StringVarP(&metricsAddr, "metrics", "m", "", "prometheus client address to bind of format <host>:<port>")
	rootCmd.Flags().StringVarP(&logLevel, "log", "l", logrus.InfoLevel.String(), fmt.Sprintf("log level %v", logrus.AllLevels))
	rootCmd.Flags().DurationVar(&entryTimeout, "entry-timeout", store.DefaultTimeout, "discovery entry timeout")
	rootCmd.Flags().BoolVarP(&testMode, "test-mode", "t", false, "in testing mode")
}

var rootCmd = &cobra.Command{
	Use:   "dmsg-discovery",
	Short: "Dmsg Discovery Server for skywire",
	Run: func(_ *cobra.Command, _ []string) {
		log, out := prepareLogger()

		if _, err := buildinfo.Get().WriteTo(out); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		db := prepareDB(log)
		m := prepareMetrics(log)
		a := api.New(log, db, testMode)

		log.WithField("addr", addr).Info("Serving discovery API...")
		log.Fatal(http.ListenAndServe(addr, m.Handle(a)))
	},
}

func prepareLogger() (logrus.FieldLogger, io.Writer) {
	mLog := logging.NewMasterLogger()

	lvl, err := logrus.ParseLevel(logLevel)
	if err != nil {
		mLog.Fatalf("Invalid log level: %v", logLevel)
	}
	mLog.SetLevel(lvl)

	if syslogAddr != "" {
		hook, err := logrussyslog.NewSyslogHook("udp", syslogAddr, syslog.LOG_INFO, tag)
		if err != nil {
			log.Fatalf("Unable to connect to syslog daemon on %v", syslogAddr)
		}
		mLog.AddHook(hook)
	}

	return mLog.PackageLogger(tag), mLog.Out
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

func prepareMetrics(log logrus.FieldLogger) promutil.HTTPMetrics {
	if metricsAddr == "" {
		return promutil.NewEmptyHTTPMetrics()
	}

	m := promutil.NewHTTPMetrics(tag)

	mux := http.NewServeMux()
	promutil.AddMetricsHandle(mux, m.Collectors()...)

	log.WithField("addr", metricsAddr).Info("Serving metrics...")
	go func() { log.Fatal(http.ListenAndServe(metricsAddr, mux)) }()

	return m
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
