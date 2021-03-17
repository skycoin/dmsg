package commands

import (
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var unsafe = false

func init() {
	confgenCmd.Flags().BoolVar(&unsafe, "unsafe", unsafe,
		"will unsafely write config if set")

	rootCmd.AddCommand(confgenCmd)
}

var confgenCmd = &cobra.Command{
	Use:    "confgen <config.json>",
	Short:  "generates config file",
	Args:   cobra.ExactArgs(1),
	PreRun: prepareVariables,
	RunE: func(cmd *cobra.Command, args []string) error {
		if unsafe {
			return viper.WriteConfigAs(args[0])
		}
		return viper.SafeWriteConfigAs(args[0])
	},
}
