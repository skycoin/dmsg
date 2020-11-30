package commands

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cmd/dmsg-discovery/internal/api"
	"github.com/skycoin/dmsg/cmd/dmsg-discovery/internal/store"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/discord"
	"github.com/skycoin/dmsg/resourcemonitor"
)

const redisPasswordEnvName = "REDIS_PASSWORD"

var (
	sf                cmdutil.ServiceFlags
	addr              string
	redisURL          string
	entryTimeout      time.Duration
	testMode          bool
	enableLoadTesting bool
)

func init() {
	sf.Init(rootCmd, "dmsg_disc", "")

	rootCmd.Flags().StringVarP(&addr, "addr", "a", ":9090", "address to bind to")
	rootCmd.Flags().StringVar(&redisURL, "redis", store.DefaultURL, "connections string for a redis store")
	rootCmd.Flags().DurationVar(&entryTimeout, "entry-timeout", store.DefaultTimeout, "discovery entry timeout")
	rootCmd.Flags().BoolVarP(&testMode, "test-mode", "t", false, "in testing mode")
	rootCmd.Flags().BoolVar(&enableLoadTesting, "enable-load-testing", false, "enable load testing")
}

var rootCmd = &cobra.Command{
	Use:   "dmsg-discovery",
	Short: "Dmsg Discovery Server for skywire",
	Run: func(_ *cobra.Command, _ []string) {
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

		m := sf.HTTPMetrics()

		db := prepareDB(log)

		a := api.New(log, db, testMode, enableLoadTesting)

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		mon := resourcemonitor.New(log, resourcemonitor.DefaultOptions)
		mon.StartInBackground(ctx)

		go a.RunBackgroundTasks(ctx, log)

		log.WithField("addr", addr).Info("Serving discovery API...")
		go func() {
			if err := http.ListenAndServe(addr, m.Handle(a)); err != nil {
				log.Errorf("ListenAndServe: %v", err)
				cancel()
			}
		}()
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

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
