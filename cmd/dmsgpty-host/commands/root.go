package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path"
	"sync"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/disc"
	"github.com/SkycoinProject/dmsg/dmsgpty"
)

const defaultEnvPrefix = "DMSGPTY"

var log = logging.MustGetLogger("dmsgpty-host:init")

// variables
var (
	// persistent flags (with viper references)
	skGen        = false
	sk           cipher.SecKey
	wlPath       = ""
	dmsgDisc     = dmsg.DefaultDiscAddr
	dmsgSessions = dmsg.DefaultMinSessions
	dmsgPort     = dmsgpty.DefaultPort
	cliNet       = dmsgpty.DefaultCLINet
	cliAddr      = dmsgpty.DefaultCLIAddr

	// persistent flags (without viper references)
	envPrefix = defaultEnvPrefix

	// root command flags (without viper references)
	confStdin = false
	confPath  = ""
)

// init prepares flags.
// Some flags are persistent, and some need to be bound with env/config references (via viper).
func init() {

	// Prepare flags with env/config references.
	// We will bind flags to associated viper values so that they can be set with envs and config file.

	rootCmd.PersistentFlags().BoolVar(&skGen, "skgen", skGen,
		"if set, a random secret key will be generated")

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

	cmdutil.Catch(viper.BindPFlags(rootCmd.PersistentFlags())) // Bind above flags with env/config references.

	// Prepare flags without associated env/config references.

	rootCmd.PersistentFlags().StringVar(&envPrefix, "envprefix", envPrefix,
		"env prefix")

	rootCmd.Flags().BoolVar(&confStdin, "confstdin", confStdin,
		"config will be read from stdin if set")

	rootCmd.Flags().StringVar(&confPath, "confpath", confPath,
		"config path")
}

// prepareVariables sources variables in the following precedence order: flags, env, config, default.
//
// The following actions are performed:
// - Prepare how envs are sourced.
// - Prepare how config is to be sourced.
// - Grab final values of variables.
//		Viper uses the following precedence order: flags, env, config, default.
//		Source: https://github.com/spf13/viper#why-viper
//
// Panics are called via `cmdutil.Catch` or `cmdutil.CatchWithMsg`.
// These are recovered in a defer statement where the help message is printed.
func prepareVariables(cmd *cobra.Command, _ []string) {

	// Recover and print help on panic.
	defer func() {
		if r := recover(); r != nil {
			cmd.PrintErrln("Error:", r)
			fmt.Print("Help:\n  ")
			if err := cmd.Help(); err != nil {
				panic(err)
			}
			os.Exit(1)
		}
	}()

	// Prepare how ENVs are sourced.
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()

	// Prepare how config file is sourced (if root command).
	if cmd.Name() == rootCmdName() {
		viper.SetConfigName("config")
		viper.SetConfigType("json")
		if confStdin {
			v := make(map[string]interface{})
			buf := new(bytes.Buffer)
			cmdutil.CatchWithMsg("flag 'confstdin' is set, but config read from stdin is invalid",
				json.NewDecoder(os.Stdin).Decode(&v),
				json.NewEncoder(buf).Encode(v),
				viper.ReadConfig(buf))
		} else if confPath != "" {
			viper.SetConfigFile(confPath)
			cmdutil.CatchWithMsg("flag 'confpath' is set, but we failed to read config from specified path",
				viper.ReadInConfig())
		}
	}

	// Grab final values of variables.

	// Grab secret key (from 'sk' and 'skgen' flags).
	if skGen = viper.GetBool("skgen"); skGen {
		if !sk.Null() {
			log.Fatal("Values 'skgen' and 'sk' cannot be both set.")
		}
		var pk cipher.PubKey
		pk, sk = cipher.GenerateKeyPair()
		log.WithField("pubkey", pk).
			WithField("seckey", sk).
			Info("Generating key pair as 'skgen' is set.")
		viper.Set("sk", sk)
	}
	skStr := viper.GetString("sk")
	cmdutil.CatchWithMsg("value 'seckey' is invalid", sk.Set(skStr))

	wlPath = viper.GetString("wl")
	dmsgDisc = viper.GetString("dmsgdisc")
	dmsgSessions = viper.GetInt("dmsgsessions")
	dmsgPort = cast.ToUint16(viper.Get("dmsgport"))
	cliNet = viper.GetString("clinet")
	cliAddr = viper.GetString("cliaddr")

	// Print values.
	pLog := logrus.FieldLogger(log)
	cmd.Flags().VisitAll(func(flag *pflag.Flag) {
		if v := viper.Get(flag.Name); v != nil {
			pLog = pLog.WithField(flag.Name, v)
		}
	})
	pLog.Info("Init complete.")
}

func rootCmdName() string {
	return path.Base(os.Args[0])
}

var rootCmd = &cobra.Command{
	Use:    rootCmdName(),
	Short:  "runs a standalone dmsgpty-host instance",
	PreRun: prepareVariables,
	Run: func(cmd *cobra.Command, args []string) {
		log := logging.MustGetLogger("dmsgpty-host")

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		pk, err := sk.PubKey()
		cmdutil.CatchWithLog(log, "failed to derive public key from secret key", err)

		// Prepare and serve dmsg client and wait until ready.
		dmsgC := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc), &dmsg.Config{
			MinSessions: dmsgSessions,
		})
		go dmsgC.Serve()
		select {
		case <-ctx.Done():
			cmdutil.CatchWithLog(log, "failed to wait unti dmsg client to be ready", ctx.Err())
		case <-dmsgC.Ready():
		}

		// Prepare whitelist.
		var wl dmsgpty.Whitelist
		if wlPath == "" {
			wl = dmsgpty.NewMemoryWhitelist()
		} else {
			var err error
			wl, err = dmsgpty.NewJSONFileWhiteList(wlPath)
			cmdutil.CatchWithLog(log, "failed to init whitelist", err)
		}

		// Prepare dmsgpty host.
		host := dmsgpty.NewHost(dmsgC, wl)
		wg := new(sync.WaitGroup)
		wg.Add(2)

		// Prepare CLI.
		cliL, err := net.Listen(cliNet, cliAddr)
		cmdutil.CatchWithLog(log, "failed to serve CLI", err)
		log.WithField("addr", cliL.Addr()).Info("Listening for CLI connections.")
		go func() {
			log.WithError(host.ServeCLI(ctx, cliL)).
				Info("Stopped serving CLI.")
			wg.Done()
		}()

		// Serve dmsgpty.
		log.WithField("port", dmsgPort).
			Info("Listening for dmsg streams.")
		go func() {
			log.WithError(host.ListenAndServe(ctx, dmsgPort)).
				Info("Stopped serving dmsgpty-host.")
			wg.Done()
		}()

		wg.Wait()
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
