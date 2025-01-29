// Example Hello World HTTP over DMSG
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

var (
	sk       cipher.SecKey
	dmsgDisc string
	dmsgPort uint
)

func init() {
	RootCmd.Flags().UintVarP(&dmsgPort, "port", "p", 80, "DMSG port to serve from")
	RootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", dmsg.DiscAddr(false), "DMSG discovery URL")
	if os.Getenv("DMSGHTTP_SK") != "" {
		sk.Set(os.Getenv("DMSGHTTP_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "A random key is generated if unspecified\n\r")
}

// RootCmd contains the root DMSG HTTP command
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(os.Args[0], " ")[0]
	}(),
	Short: "DMSG HTTP Hello World server",
	Long:  "DMSG HTTP Hello World server",
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Run: func(_ *cobra.Command, _ []string) {
		log := logging.MustGetLogger("dmsghttp")
		if dmsgDisc == "" {
			log.Fatal("DMSG discovery URL not specified")
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		// Generate keys if not provided
		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}

		// Initialize the DMSG client
		c := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
		defer func() {
			if err := c.Close(); err != nil {
				log.WithError(err).Error("Failed to close DMSG client")
			}
		}()
		go c.Serve(context.Background())

		// Wait for the DMSG client to be ready
		select {
		case <-ctx.Done():
			log.WithError(ctx.Err()).Warn()
			return
		case <-c.Ready():
		}

		// Listen on the specified DMSG port
		lis, err := c.Listen(uint16(dmsgPort))
		if err != nil {
			log.WithError(err).Fatal("Failed to listen on DMSG port")
		}
		defer lis.Close()

		log.Infof("Serving Hello World on DMSG address %s", lis.Addr())

		// Set up HTTP server to respond with "Hello, World!"
		http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
			log.Infof("Received request from %s", r.RemoteAddr)
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("Hello, World!"))
		})

		// Start the HTTP server
		server := &http.Server{
			ReadHeaderTimeout: 3 * time.Second,
		}

		// Graceful shutdown handler
		go func() {
			<-ctx.Done()
			log.Info("Shutdown signal received, shutting down HTTP server...")
			server.Shutdown(context.Background())
			log.Info("HTTP server successfully shut down")
		}()

		// Start serving HTTP requests
		log.Fatal(server.Serve(lis))
	},
}

// Execute executes the root CLI command
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}

func init() {
	var helpflag bool
	RootCmd.SetUsageTemplate(help)
	RootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for dmsghttp-cli")
	RootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	RootCmd.PersistentFlags().MarkHidden("help") //nolint
}

func main() {
	cc.Init(&cc.Config{
		RootCmd:         RootCmd,
		Headings:        cc.HiBlue + cc.Bold,
		Commands:        cc.HiBlue + cc.Bold,
		CmdShortDescr:   cc.HiBlue,
		Example:         cc.HiBlue + cc.Italic,
		ExecName:        cc.HiBlue + cc.Bold,
		Flags:           cc.HiBlue + cc.Bold,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})
	Execute()
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
