package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/fsutil"
)

var unsafe = false

func init() {
	confgenCmd.Flags().BoolVar(&unsafe, "unsafe", unsafe,
		"will unsafely write config if set")

	rootCmd.AddCommand(confgenCmd)
}

func writeConfig(conf config, path string) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed to open config file: %w", err)
	}
	enc := json.NewEncoder(f)
	enc.SetIndent("", "    ")
	return enc.Encode(&conf)
}

var confgenCmd = &cobra.Command{
	Use:    "confgen <config.json>",
	Short:  "generates config file",
	Args:   cobra.MaximumNArgs(1),
	PreRun: func(cmd *cobra.Command, args []string) {},
	RunE: func(cmd *cobra.Command, args []string) error {

		exists, err := fsutil.Exists(confPath)
		if err != nil {
			return fmt.Errorf("failed to check if config file exists: %w", err)
		}
		if exists {
			return fmt.Errorf("config file %s already exists", confPath)
		}

		if len(args) == 0 {
			confPath = "./config.json"
		} else {
			confPath = args[0]
		}
		conf, err := getConfig(cmd)
		if err != nil {
			return fmt.Errorf("failed to get config: %w", err)
		}

		if unsafe {
			return writeConfig(conf, confPath)
		}

		return writeConfig(conf, confPath)
	},
}
