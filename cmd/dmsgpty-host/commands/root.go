package commands

import (
	"context"
	"fmt"
	stdlog "log"
	"net"
	"os"
	"strconv"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/sirupsen/logrus"
	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/disc"
	"github.com/skycoin/dmsg/dmsgpty"
)

const defaultEnvPrefix = "DMSGPTY"

var log = logging.MustGetLogger("dmsgpty-host:init")

var json = jsoniter.ConfigFastest

// variables
var (
	// persistent flags
	sk           cipher.SecKey
	wlPath       = ""
	dmsgDisc     = dmsg.DefaultDiscAddr
	dmsgSessions = dmsg.DefaultMinSessions
	dmsgPort     = dmsgpty.DefaultPort
	cliNet       = dmsgpty.DefaultCLINet
	cliAddr      = dmsgpty.DefaultCLIAddr

	// persistent flags
	skGen     = false
	envPrefix = defaultEnvPrefix

	// root command flags
	confStdin = false
	confPath  = ""
)

// init prepares flags.
func init() {

	// Prepare flags with env/config references.

	rootCmd.PersistentFlags().Var(&sk, "sk",
		"secret key of the dmsgpty-host")

	rootCmd.PersistentFlags().StringVar(&wlPath, "wl", wlPath,
		"path of json whitelist file (if unspecified, a memory whitelist will be used)")

	rootCmd.PersistentFlags().StringVar(&dmsgDisc, "dmsgdisc", dmsgDisc,
		"dmsg discovery address")

	rootCmd.PersistentFlags().IntVar(&dmsgSessions, "dmsgsessions", dmsgSessions,
		"minimum number of dmsg sessions to ensure")

	rootCmd.PersistentFlags().Uint16Var(&dmsgPort, "dmsgport", dmsgPort,
		"dmsg port for listening for remote hosts")

	rootCmd.PersistentFlags().StringVar(&cliNet, "clinet", cliNet,
		"network used for listening for cli connections")

	rootCmd.PersistentFlags().StringVar(&cliAddr, "cliaddr", cliAddr,
		"address used for listening for cli connections")

	// Prepare flags without associated env/config references.

	rootCmd.PersistentFlags().BoolVar(&skGen, "skgen", skGen,
		"if set, a random secret key will be generated")

	rootCmd.PersistentFlags().StringVar(&envPrefix, "envprefix", envPrefix,
		"env prefix")

	rootCmd.Flags().BoolVar(&confStdin, "confstdin", confStdin,
		"config will be read from stdin if set")

	rootCmd.Flags().StringVar(&confPath, "confpath", confPath,
		"config path")
}

type config struct {
	SK           cipher.SecKey `json:"-"`
	SKStr        string        `json:"sk"`
	WLPath       string        `json:"wl"`
	DmsgDisc     string        `json:"dmsgdisc"`
	DmsgSessions int           `json:"dmsgsessions"`
	DmsgPort     uint16        `json:"dmsgport"`
	CLINet       string        `json:"clinet"`
	CLIAddr      string        `json:"cliaddr"`
}

func defaultConfig() config {
	return config{
		DmsgDisc:     dmsg.DefaultDiscAddr,
		DmsgSessions: dmsg.DefaultMinSessions,
		DmsgPort:     dmsgpty.DefaultPort,
		CLINet:       dmsgpty.DefaultCLINet,
		CLIAddr:      dmsgpty.DefaultCLIAddr,
	}
}

func configFromJSON(conf config) (config, error) {
	var jsonConf config

	if confStdin {
		if err := json.NewDecoder(os.Stdin).Decode(&jsonConf); err != nil {
			return config{}, fmt.Errorf("flag 'confstdin' is set, but config read from stdin is invalid: %w", err)
		}
	}

	if confPath != "" {
		f, err := os.Open(confPath)
		if err != nil {
			return config{}, fmt.Errorf("failed to open config file: %w", err)
		}

		if err := json.NewDecoder(f).Decode(&jsonConf); err != nil {
			return config{}, fmt.Errorf("flag 'confpath' is set, but we failed to read config from specified path: %w", err)
		}
	}

	if jsonConf.SKStr != "" {
		if err := jsonConf.SK.Set(jsonConf.SKStr); err != nil {
			return config{}, fmt.Errorf("provided SK is invalid: %w", err)
		}
	}

	if !jsonConf.SK.Null() {
		conf.SKStr = jsonConf.SKStr
		conf.SK = jsonConf.SK
	}

	if jsonConf.WLPath != "" {
		conf.WLPath = jsonConf.WLPath
	}

	if jsonConf.DmsgDisc != "" {
		conf.DmsgDisc = jsonConf.DmsgDisc
	}

	if conf.DmsgSessions != 0 {
		conf.DmsgSessions = jsonConf.DmsgSessions
	}

	if conf.DmsgPort != 0 {
		conf.DmsgPort = jsonConf.DmsgPort
	}

	if conf.CLINet != "" {
		conf.CLINet = jsonConf.CLINet
	}

	if conf.CLIAddr != "" {
		conf.CLIAddr = jsonConf.CLIAddr
	}

	return conf, nil
}

func fillConfigFromENV(conf config) (config, error) {
	if val, ok := os.LookupEnv(envPrefix + "_SK"); ok {
		if err := conf.SK.Set(val); err != nil {
			return conf, fmt.Errorf("provided SK is invalid: %w", err)
		}

		conf.SKStr = val
	}

	if val, ok := os.LookupEnv(envPrefix + "_WL"); ok {
		conf.WLPath = val
	}

	if val, ok := os.LookupEnv(envPrefix + "_DMSGDISC"); ok {
		conf.DmsgDisc = val
	}

	if val, ok := os.LookupEnv(envPrefix + "_DMSGSESSIONS"); ok {
		dmsgSessions, err := strconv.Atoi(val)
		if err != nil {
			return conf, fmt.Errorf("failed to parse dmsg sessions: %w", err)
		}

		conf.DmsgSessions = dmsgSessions
	}

	if val, ok := os.LookupEnv(envPrefix + "_DMSGPORT"); ok {
		dmsgPort, err := strconv.ParseUint(val, 10, 16)
		if err != nil {
			return conf, fmt.Errorf("failed to parse dmsg port: %w", err)
		}

		conf.DmsgPort = uint16(dmsgPort)
	}

	if val, ok := os.LookupEnv(envPrefix + "_CLINET"); ok {
		conf.CLINet = val
	}

	if val, ok := os.LookupEnv(envPrefix + "_CLIADDR"); ok {
		conf.CLIAddr = val
	}

	return conf, nil
}

func fillConfigFromFlags(conf config) config {
	if !sk.Null() {
		conf.SKStr = sk.Hex()
		conf.SK = sk
	}

	if wlPath != "" {
		conf.WLPath = wlPath
	}

	if dmsgDisc != dmsg.DefaultDiscAddr {
		conf.DmsgDisc = dmsgDisc
	}

	if dmsgSessions != dmsg.DefaultMinSessions {
		conf.DmsgSessions = dmsgSessions
	}

	if dmsgPort != dmsgpty.DefaultPort {
		conf.DmsgPort = dmsgPort
	}

	if cliNet != dmsgpty.DefaultCLINet {
		conf.CLINet = cliNet
	}

	if cliAddr != dmsgpty.DefaultCLIAddr {
		conf.CLIAddr = cliAddr
	}

	return conf
}

// getConfig sources variables in the following precedence order: flags, env, config, default.
func getConfig(cmd *cobra.Command) (config, error) {
	conf := defaultConfig()

	var err error

	// Prepare how config file is sourced (if root command).
	if cmd.Name() == cmdutil.RootCmdName() {
		conf, err = configFromJSON(conf)
		if err != nil {
			return config{}, fmt.Errorf("failed to read config from JSON: %w", err)
		}
	}

	conf, err = fillConfigFromENV(conf)
	if err != nil {
		return conf, fmt.Errorf("failed to fill config from ENV: %w", err)
	}

	// Grab secret key (from 'sk' and 'skgen' flags).
	if skGen {
		if !sk.Null() {
			log.Fatal("Values 'skgen' and 'sk' cannot be both set.")
		}
		var pk cipher.PubKey
		pk, sk = cipher.GenerateKeyPair()
		log.WithField("pubkey", pk).
			WithField("seckey", sk).
			Info("Generating key pair as 'skgen' is set.")

		conf.SKStr = sk.Hex()
		if err := conf.SK.Set(conf.SKStr); err != nil {
			return conf, err
		}
	}

	conf = fillConfigFromFlags(conf)

	if conf.SK.Null() {
		return conf, fmt.Errorf("value 'seckey' is invalid")
	}

	// Print values.
	pLog := logrus.FieldLogger(log)
	pLog = pLog.WithField("sk", conf.SK)
	pLog = pLog.WithField("wl", conf.WLPath)
	pLog = pLog.WithField("dmsgdisc", conf.DmsgDisc)
	pLog = pLog.WithField("dmsgsessions", conf.DmsgSessions)
	pLog = pLog.WithField("dmsgport", conf.DmsgPort)
	pLog = pLog.WithField("clinet", conf.CLINet)
	pLog = pLog.WithField("cliaddr", conf.CLIAddr)
	pLog.Info("Init complete.")

	return conf, nil
}

var rootCmd = &cobra.Command{
	Use:    cmdutil.RootCmdName(),
	Short:  "runs a standalone dmsgpty-host instance",
	PreRun: func(cmd *cobra.Command, args []string) {},
	RunE: func(cmd *cobra.Command, args []string) error {
		conf, err := getConfig(cmd)
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if _, err := buildinfo.Get().WriteTo(stdlog.Writer()); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		log := logging.MustGetLogger("dmsgpty-host")

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		pk, err := conf.SK.PubKey()
		if err != nil {
			return fmt.Errorf("failed to derive public key from secret key: %w", err)
		}

		// Prepare and serve dmsg client and wait until ready.
		dmsgC := dmsg.NewClient(pk, conf.SK, disc.NewHTTP(conf.DmsgDisc), &dmsg.Config{
			MinSessions: conf.DmsgSessions,
		})
		go dmsgC.Serve(context.Background())
		select {
		case <-ctx.Done():
			return fmt.Errorf("failed to wait dmsg client to be ready: %w", ctx.Err())
		case <-dmsgC.Ready():
		}

		// Prepare whitelist.
		var wl dmsgpty.Whitelist
		if conf.WLPath == "" {
			wl = dmsgpty.NewMemoryWhitelist()
		} else {
			var err error
			wl, err = dmsgpty.NewJSONFileWhiteList(conf.WLPath)
			if err != nil {
				return fmt.Errorf("failed to init whitelist: %w", err)
			}
		}

		// Prepare dmsgpty host.
		host := dmsgpty.NewHost(dmsgC, wl)
		wg := new(sync.WaitGroup)
		wg.Add(2)

		// Prepare CLI.
		if conf.CLINet == "unix" {
			_ = os.Remove(conf.CLIAddr) //nolint:errcheck
		}
		cliL, err := net.Listen(conf.CLINet, conf.CLIAddr)
		if err != nil {
			return fmt.Errorf("failed to serve CLI: %w", err)
		}
		log.WithField("addr", cliL.Addr()).Info("Listening for CLI connections.")
		go func() {
			log.WithError(host.ServeCLI(ctx, cliL)).
				Info("Stopped serving CLI.")
			wg.Done()
		}()

		// Serve dmsgpty.
		log.WithField("port", conf.DmsgPort).
			Info("Listening for dmsg streams.")
		go func() {
			log.WithError(host.ListenAndServe(ctx, conf.DmsgPort)).
				Info("Stopped serving dmsgpty-host.")
			wg.Done()
		}()

		wg.Wait()

		return nil
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
