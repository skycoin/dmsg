package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/cipher"
)

func init() {
	rootCmd.AddCommand(
		whitelistCmd,
		whitelistAddCmd,
		whitelistRemoveCmd)
}

var whitelistCmd = &cobra.Command{
	Use:   "whitelist",
	Short: "lists all whitelisted public keys",
	RunE: func(cmd *cobra.Command, args []string) error {

		wlC, err := cli.WhitelistClient()
		if err != nil {
			return err
		}
		pks, err := wlC.ViewWhitelist()
		if err != nil {
			return err
		}
		for _, pk := range pks {
			fmt.Println(pk)
		}
		return nil
	},
}

var whitelistAddCmd = &cobra.Command{
	Use:   "whitelist-add <public-key>...",
	Short: "adds public key(s) to the whitelist",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {

		pks, err := pksFromArgs(args)
		if err != nil {
			return err
		}

		// append new pks to the whitelist slice within the config file
		for _, k := range pks {
			conf.Wl = append(conf.Wl, k)
		}

		// write the changes back to the config file
		updateFile()

		wlC, err := cli.WhitelistClient()
		if err != nil {
			return err
		}
		return wlC.WhitelistAdd(pks...)
	},
}

var whitelistRemoveCmd = &cobra.Command{
	Use:   "whitelist-remove <public-key>...",
	Short: "removes public key(s) from the whitelist",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(_ *cobra.Command, args []string) error {
		pks, err := pksFromArgs(args)
		if err != nil {
			return err
		}
		wlC, err := cli.WhitelistClient()
		if err != nil {
			return err
		}
		return wlC.WhitelistRemove(pks...)
	},
}

func pksFromArgs(args []string) ([]cipher.PubKey, error) {
	pks := make([]cipher.PubKey, len(args))
	for i, str := range args {
		if err := pks[i].Set(str); err != nil {
			return nil, fmt.Errorf("failed to parse public key at index %d: %v", i, err)
		}
	}
	return pks, nil
}

// func update config file
func updateFile() error {

	// marshal content
	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	// show changed config
	os.Stdout.Write(b)

	// write to config.json
	err = ioutil.WriteFile("config.json", b, 0644)
	if err != nil {
		return err
	}

	return nil
}
