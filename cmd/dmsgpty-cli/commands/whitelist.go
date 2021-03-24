package commands

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"

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

		// duplicate flag
		var dFlag bool

		// append new pks to the whitelist slice within the config file

		// for each pk to be added
		for _, k := range pks {

			dFlag = false

			// check if the pk already exists
			for _, p := range conf.Wl {

				// if it does
				if p == k {
					// flag it
					dFlag = true
					fmt.Printf("skipping append for %v. Already exists", k)
					break
				}
			}

			// if pk does already not exist
			if !dFlag {
				// append it
				conf.Wl = append(conf.Wl, k)
			}

		}

		// write the changes back to the config file
		err = updateFile()
		if err != nil {
			log.Println("unable to update config file")
			return err
		}

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

		// for each pubkey to be removed
		for _, k := range pks {

			// find occurrence of pubkey in config whitelist
			for i := 0; i < len(conf.Wl); i++ {

				// if an occurrence is found
				if k == conf.Wl[i] {
					// remove element
					conf.Wl = append(conf.Wl[:i], conf.Wl[i+1:]...)
					break
				}
			}
		}

		// write changes back to the config file
		err = updateFile()
		if err != nil {
			log.Println("unable to update config file")
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

// updateFile writes changes to config file
func updateFile() error {

	// marshal content
	b, err := json.MarshalIndent(conf, "", "  ")
	if err != nil {
		return err
	}

	// (optionally) display changed config
	// _, err = os.Stdout.Write(b)
	// if err != nil {
	//	log.Println("unable to write to stdout")
	//	return err
	// }

	// write to config file
	err = ioutil.WriteFile("config.json", b, 0600)
	if err != nil {
		return err
	}

	return nil
}
