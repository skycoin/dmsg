package commands

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg"
	"github.com/skycoin/dmsg/buildinfo"
	"github.com/skycoin/dmsg/cipher"
	"github.com/skycoin/dmsg/cmdutil"
	"github.com/skycoin/dmsg/dmsgpty"
)

type config struct {
	CLIAddr      string         `json:"cliaddr"`
	CLINet       string         `json:"clinet"`
	DmsgDisc     string         `json:"dmsgdisc"`
	DmsgPort     uint16         `json:"dmsgport"`
	DmsgSessions int            `json:"dmsgsessions"`
	SK           cipher.SecKey  `json:"-"`
	SKStr        string         `json:"sk"`
	Wl           cipher.PubKeys `json:"wl"`
}

func defaultConfig() config {
	return config{
		DmsgDisc:     dmsg.DefaultDiscAddr,
		DmsgSessions: dmsg.DefaultMinSessions,
		DmsgPort:     dmsgpty.DefaultPort,
		CLINet:       dmsgpty.DefaultCLINet,
		CLIAddr:      dmsgpty.DefaultCLIAddr,
	}
}

var cli = dmsgpty.DefaultCLI()

func init() {
	rootCmd.PersistentFlags().StringVar(&cli.Net, "clinet", cli.Net,
		"network to use for dialing to dmsgpty-host")

	rootCmd.PersistentFlags().StringVar(&cli.Addr, "cliaddr", cli.Addr,
		"address to use for dialing to dmsgpty-host")

	rootCmd.PersistentFlags().StringVar(&confPath, "confpath", confPath,
		"config path")
}

// conf to update whitelists
var conf config = defaultConfig()

// path for config file ( required for whitelists )
var confPath = "config.json"

var remoteAddr dmsg.Addr
var cmdName = dmsgpty.DefaultCmd
var cmdArgs []string

func init() {

	cobra.OnInitialize(initConfig)
	rootCmd.Flags().Var(&remoteAddr, "addr",
		"remote dmsg address of format 'pk:port'. If unspecified, the pty will start locally")

	rootCmd.Flags().StringVarP(&cmdName, "cmd", "c", cmdName,
		"name of command to run")

	rootCmd.Flags().StringSliceVarP(&cmdArgs, "args", "a", cmdArgs,
		"command arguments")

}

func initConfig() {

	// check if "config.json" exists
	// confPath := "./config.json"
	err := checkFile(confPath)
	if err != nil {
		cli.Log.Fatalln("Default config file \"config.json\" not found.")
	}

	// read file using ioutil
	file, err := ioutil.ReadFile(confPath)
	if err != nil {
		cli.Log.Fatalln("Unable to read ", confPath, err)
	}

	// store config.json into conf
	err = json.Unmarshal(file, &conf)
	if err != nil {
		cli.Log.Errorln(err)
		// ignoring this error
	}

	// check if config file is newly created
	if conf.Wl == nil {

		// if so, add a whitelist slice field "wl"
		// marshal content
		b, err := json.MarshalIndent(conf, "", "  ")
		if err != nil {
			cli.Log.Fatalln("Unable to marshal conf")
		}

		// show changed config
		// _, err = os.Stdout.Write(b)
		// if err != nil {
		// 	cli.Log.Info("unable to write to stdout")

		// }

		// write to config.json
		err = ioutil.WriteFile("config.json", b, 0600)
		if err != nil {
			cli.Log.Fatalln("Unable to write", confPath, err)
		}
	}
}

var rootCmd = &cobra.Command{
	Use:   "dmsgpty-cli",
	Short: "Run commands over dmsg",
	PreRun: func(*cobra.Command, []string) {
		if remoteAddr.Port == 0 {
			remoteAddr.Port = dmsgpty.DefaultPort
		}
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		if _, err := buildinfo.Get().WriteTo(log.Writer()); err != nil {
			log.Printf("Failed to output build info: %v", err)
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), nil)
		defer cancel()

		if remoteAddr.PK.Null() {
			// Local pty.
			return cli.StartLocalPty(ctx, cmdName, cmdArgs...)
		}
		// Remote pty.
		return cli.StartRemotePty(ctx, remoteAddr.PK, remoteAddr.Port, cmdName, cmdArgs...)
	},
}

// Execute executes the root command.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

// func read config file
func checkFile(confPath string) error {
	_, err := os.Stat(confPath)
	if os.IsNotExist(err) {
		_, err := os.Create(confPath)
		if err != nil {
			return err
		}
	}
	return nil
}
