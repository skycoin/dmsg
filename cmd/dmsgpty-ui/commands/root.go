package commands

import (
	"net/http"
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/SkycoinProject/dmsg/cmdutil"
	"github.com/SkycoinProject/dmsg/dmsgpty"
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

	rootCmd.PersistentFlags().IntVar(&conf.TermCols, "cols", conf.TermCols,
		"default number of columns across for terminals")

	rootCmd.PersistentFlags().IntVar(&conf.TermRows, "rows", conf.TermRows,
		"default number of columns across for terminals")
}

var rootCmd = &cobra.Command{
	Use:   cmdutil.RootCmdName(),
	Short: "hosts a UI server for a dmsgpty-host",
	Run: func(cmd *cobra.Command, args []string) {
		hostDialer := dmsgpty.NetUIDialer(hostNet, hostAddr)
		ui := dmsgpty.NewUI(hostDialer, conf)

		logrus.
			WithError(http.ListenAndServe(addr, ui.UIServeMux("/dmsgpty"))).
			Info("Stopped serving.")
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
