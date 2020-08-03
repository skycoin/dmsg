package commands

import (
	"log"
	"net/http"
	"os"

	"github.com/SkycoinProject/skywire-mainnet/pkg/util/buildinfo"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/dmsgpty"
)

var (
	hostNet  = dmsgpty.DefaultCLINet
	hostAddr = dmsgpty.DefaultCLIAddr
	addr     = ":8080"
	conf     = dmsgpty.DefaultUIConfig()
)

func init() {
	rootCmd.PersistentFlags().StringVar(&hostNet, "hnet", hostNet,
		"dmsgpty-host network name")

	rootCmd.PersistentFlags().StringVar(&hostAddr, "haddr", hostAddr,
		"dmsgpty-host network address")

	rootCmd.PersistentFlags().StringVar(&addr, "addr", addr,
		"network address to serve UI on")

	rootCmd.PersistentFlags().StringVar(&conf.CmdName, "cmd", conf.CmdName,
		"command to run when initiating pty")

	rootCmd.PersistentFlags().StringArrayVar(&conf.CmdArgs, "arg", conf.CmdArgs,
		"command arguments to include when initiating pty")
}

var rootCmd = &cobra.Command{
	Use:   cmdutil.RootCmdName(),
	Short: "hosts a UI server for a dmsgpty-host",
	Run: func(cmd *cobra.Command, args []string) {
		if _, err := buildinfo.Get().WriteTo(log.Writer()); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		ui := dmsgpty.NewUI(dmsgpty.NetUIDialer(hostNet, hostAddr), conf)
		logrus.
			WithField("addr", addr).
			Info("Serving.")

		err := http.ListenAndServe(addr, ui.Handler())
		logrus.
			WithError(err).
			Info("Stopped serving.")
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
