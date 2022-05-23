package commands

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/cmd/dmsg-server/commands/config"
	"github.com/skycoin/dmsg/cmd/dmsg-server/commands/start"
)

var rootCmd = &cobra.Command{
	Use:   "dmsg-server",
	Short: "Command Line Interface for DMSG-Server",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐   ┌─┐┌─┐┬─┐┬  ┬┌─┐┬─┐
	││││││└─┐│ ┬ ─ └─┐├┤ ├┬┘└┐┌┘├┤ ├┬┘
	─┴┘┴ ┴└─┘└─┘   └─┘└─┘┴└─ └┘ └─┘┴└─`,
}

func init() {
	rootCmd.AddCommand(
		config.RootCmd,
		start.RootCmd,
	)
}

// Execute executes root CLI command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}
