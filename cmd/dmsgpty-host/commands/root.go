package commands

import (
	"fmt"
	"log"

	"github.com/SkycoinProject/dmsg"
	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/dmsgpty"
	"github.com/spf13/cast"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

const defaultEnvPrefix = "DMSGPTY"

func init() {
	viper.SetConfigType("json")
	viper.SetEnvPrefix(envPrefix)
	viper.AutomaticEnv()
}

var (
	envPrefix     = defaultEnvPrefix
	configPath    string
)

func init() {
	rootCmd.Flags().StringVar(&envPrefix, "envprefix", envPrefix,
		"env prefix")

	rootCmd.Flags().StringVar(&configPath, "config", configPath,
		"config path")
}

var (
	sk            cipher.SecKey
	discAddr      = dmsg.DefaultDiscAddr
	minSessions   = dmsg.DefaultMinSessions
	whitelistPath string
	cliNet        = dmsgpty.DefaultCLINet
	cliAddr       = dmsgpty.DefaultCLIAddr
	dmsgPort      = dmsgpty.DefaultPort
)

func init() {
	rootCmd.Flags().Var(&sk, "seckey",
		"secret key of the dmsgpty-host")

	rootCmd.Flags().StringVar(&discAddr,"disc", discAddr,
		"dmsg discovery address")

	rootCmd.Flags().IntVar(&minSessions, "sessions", minSessions,
		"minimum number of dmsg sessions to ensure")

	rootCmd.Flags().StringVar(&whitelistPath, "whitelist", whitelistPath,
		"path of json whitelist file (if unspecified, a memory whitelist will be used)")

	rootCmd.Flags().StringVar(&cliNet, "net", cliNet,
		"network used for listening for cli connections")

	rootCmd.Flags().StringVar(&cliAddr, "addr", cliAddr,
		"address used for listening for cli connections")

	rootCmd.Flags().Uint16Var(&dmsgPort, "port", dmsgPort,
		"dmsg port for listening for remote hosts")

	cmdutil.Catch(viper.BindPFlags(rootCmd.Flags()))
}

func init() {
	defer func() {
		if r := recover(); r != nil {
			rootCmd.PrintErrln(r)
			fmt.Println()
			fmt.Print("Help:\n  ")
			if err := rootCmd.Help(); err != nil {
				panic(err)
			}
		}
	}()

	cmdutil.CatchWithMsg("flag 'seckey'", sk.Set(viper.GetString("seckey")))
	discAddr = viper.GetString("disc")
	minSessions = viper.GetInt("sessions")
	whitelistPath = viper.GetString("whitelist")
	cliNet = viper.GetString("net")
	cliAddr = viper.GetString("addr")
	dmsgPort = cast.ToUint16(viper.Get("port"))

	// TODO(evanlinjin): Remove.
	//cmdutil.Catch(viper.WriteConfigAs("test.json"))
}

var rootCmd = &cobra.Command{
	Use: "dmsgpty-host",
	Short: "runs a standalone dmsgpty-host instance",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println(viper.GetInt("sessions"))
		return nil
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}
