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

// struct to read / write from config
type config struct {
	SK           cipher.SecKey  `json:"-"`
	SKStr        string         `json:"sk"`
	Wl           cipher.PubKeys `json:"wl"`
	DmsgDisc     string         `json:"dmsgdisc"`
	DmsgSessions int            `json:"dmsgsessions"`
	DmsgPort     uint16         `json:"dmsgport"`
	CLINet       string         `json:"clinet"`
	CLIAddr      string         `json:"cliaddr"`
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
}

// conf to update whitelists
var conf config = defaultConfig()
var remoteAddr dmsg.Addr
var cmdName = dmsgpty.DefaultCmd
var cmdArgs []string

func init() {
	rootCmd.Flags().Var(&remoteAddr, "addr",
		"remote dmsg address of format 'pk:port'. If unspecified, the pty will start locally")

	rootCmd.Flags().StringVarP(&cmdName, "cmd", "c", cmdName,
		"name of command to run")

	rootCmd.Flags().StringSliceVarP(&cmdArgs, "args", "a", cmdArgs,
		"command arguments")

	// load config in memory to add whitelists
	// check if "config.json" exists
	filename := "config.json"
	err := checkFile(filename)
	if err != nil {
		log.Fatalln(err)
	}

	// read file using ioutil
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		log.Fatalln(err)
	}

	// store config.json into conf
	json.Unmarshal(file, &conf)

	// check if config file is newly created
	if conf.Wl == nil {

		log.Println("adding the wl slice field")

		// if so, add a whitelist slice field "wl"
		// marshal content
		b, err := json.MarshalIndent(conf, "", "  ")
		if err != nil {
			log.Fatalln(err)
		}

		// show changed config
		os.Stdout.Write(b)

		// write to config.json
		err = ioutil.WriteFile("config.json", b, 0644)
		if err != nil {
			log.Fatalln(err)
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
func checkFile(filename string) error {
	_, err := os.Stat(filename)
	if os.IsNotExist(err) {
		_, err := os.Create(filename)
		if err != nil {
			return err
		}
	}
	return nil
}
