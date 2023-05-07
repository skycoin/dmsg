// package main cmd/dmsghttp/dmsghttp.go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"

	"time"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/skycoin/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)



var (
	dmsgDisc      string
	dmsgSk        string
	serveDir string
	skString string
	pk, sk   = cipher.GenerateKeyPair()
	dmsgPort uint
)

func init() {
	skString = os.Getenv("DMSGGET_SK")
	if skString == "" {
		skString = "0000000000000000000000000000000000000000000000000000000000000000"
	}
	rootCmd.Flags().StringVarP(&serveDir, "dir", "d", ".", "local dir to serve via dmsghttp")
	rootCmd.Flags().UintVarP(&dmsgPort, "port", "p", 80, "dmsg port to serve from")
	rootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", "", "dmsg discovery url default:\n"+skyenv.DmsgDiscAddr)
	rootCmd.Flags().StringVarP(&dmsgSk, "sk", "s", "", "secret key to use default:\n"+skString)
	var helpflag bool
	rootCmd.SetUsageTemplate(help)
	rootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for "+rootCmd.Use)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().MarkHidden("help") //nolint
}



func fileServerHandler(w http.ResponseWriter, r *http.Request) {
	filePath := serveDir + r.URL.Path
	file, err := os.Open(filePath) //nolint
	if err != nil {
		fmt.Printf("%s not found\n", filePath)
		w.WriteHeader(http.StatusNotFound)
		return
	}
	defer file.Close() //nolint
	_, filename := path.Split(filePath)
	http.ServeContent(w, r, filename, time.Time{}, file)
}


var rootCmd = &cobra.Command{
	Use:   "dmsghttp",
	Short: "dmsghttp file server",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐┬ ┬┌┬┐┌┬┐┌─┐
	 │││││└─┐│ ┬├─┤ │  │ ├─┘
	─┴┘┴ ┴└─┘└─┘┴ ┴ ┴  ┴ ┴  `,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	PreRun: func(cmd *cobra.Command, args []string) {
		if dmsgDisc == "" {
			dmsgDisc = skyenv.DmsgDiscAddr
		}
		if dmsgSk == "" {
			dmsgSk = skString
		}
		//TODO: fix this
		//pk, _ = dmsgSk.PubKey()
	},
	Run: func(cmd *cobra.Command, args []string) {
		log := logging.MustGetLogger("dmsghttp")

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		c := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
		defer func() {
			if err := c.Close(); err != nil {
				log.WithError(err).Error()
			}
		}()

		go c.Serve(context.Background())

		select {
		case <-ctx.Done():
			log.WithError(ctx.Err()).Warn()
			return

		case <-c.Ready():
		}

		lis, err := c.Listen(uint16(dmsgPort))
		if err != nil {
			log.WithError(err).Fatal()
		}
		go func() {
			<-ctx.Done()
			if err := lis.Close(); err != nil {
				log.WithError(err).Error()
			}
		}()

		log.WithField("dir", serveDir).
			WithField("dmsg_addr", lis.Addr().String()).
			Info("Serving...")

		http.HandleFunc("/", fileServerHandler)

		log.Fatal(http.Serve(lis, nil))

	},
}

// Execute executes root CLI command.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:       rootCmd,
		Headings:      cc.HiBlue + cc.Bold, //+ cc.Underline,
		Commands:      cc.HiBlue + cc.Bold,
		CmdShortDescr: cc.HiBlue,
		Example:       cc.HiBlue + cc.Italic,
		ExecName:      cc.HiBlue + cc.Bold,
		Flags:         cc.HiBlue + cc.Bold,
		//FlagsDataType: cc.HiBlue,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}

const help = "Usage:\r\n" +
	"  {{.UseLine}}{{if .HasAvailableSubCommands}}{{end}} {{if gt (len .Aliases) 0}}\r\n\r\n" +
	"{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}\r\n\r\n" +
	"Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand)}}\r\n  " +
	"{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}\r\n\r\n" +
	"Flags:\r\n" +
	"{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}\r\n\r\n" +
	"Global Flags:\r\n" +
	"{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}\r\n\r\n"

func main() {
	Execute()
}
