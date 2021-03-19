package commands

import (
<<<<<<< HEAD
	"os"

=======
>>>>>>> 791525cca58af96c46a6bf00117d5a1b8d200d3c
	"github.com/skycoin/dmsg/cipher"
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
	Use:   "confgen <config.json>",
	Short: "generates config file",
<<<<<<< HEAD
	// setting max arguments to 1 so that not an passing argument follows a default set of actions
=======
	// setting max arguments to 1 so that passing flag is optional
>>>>>>> 791525cca58af96c46a6bf00117d5a1b8d200d3c
	Args:   cobra.MaximumNArgs(1),
	PreRun: prepareVariables,
	RunE: func(cmd *cobra.Command, args []string) error {

<<<<<<< HEAD
		if len(args) == 0 {
			confPath = "./config.json"
		} else {
			confPath = args[0]
		}

		if _, err := os.Stat(confPath); err == nil {

		} else if os.IsNotExist(err) {
			// generate seckey for new conf
			pk, sk := cipher.GenerateKeyPair()
			log.WithField("pubkey", pk).
				WithField("seckey", sk).
				Info("Generating key pair")

			viper.Set("sk", sk)
		}

		if unsafe {
			return viper.WriteConfigAs(confPath)
		}
		return viper.SafeWriteConfigAs(confPath)
=======
		// if no arguments are passed
		if len(args) == 0 {

			// set confPath to default
			confPath = "./config.json"

			// generate seckey
			if !skGen {
				prepareSk()
			}

			// write conf to default file
			if unsafe {
				return viper.WriteConfigAs(confPath)
			}
			return viper.SafeWriteConfigAs(confPath)

		} else {

			if unsafe {
				return viper.WriteConfigAs(args[0])
			}
			return viper.SafeWriteConfigAs(args[0])
		}
>>>>>>> 791525cca58af96c46a6bf00117d5a1b8d200d3c

	},
}

func prepareSk() {

	pk, sk := cipher.GenerateKeyPair()
	log.WithField("pubkey", pk).
		WithField("seckey", sk).
		Info("Generating key pair")
	viper.Set("sk", sk)
}
