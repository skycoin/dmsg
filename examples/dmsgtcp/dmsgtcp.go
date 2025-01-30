// Example hello world TCP over DMSG
package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/internal/cli"
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
	if os.Getenv("DMSGTCP_SK") != "" {
		sk.Set(os.Getenv("DMSGTCP_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "A random key is generated if unspecified\n\r")
}

// RootCmd contains the root DMSG TCP command
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(os.Args[0], " ")[0]
	}(),
	Short: "DMSG TCP Hello World server",
	Long:  "DMSG TCP Hello World server",
	Run: func(_ *cobra.Command, _ []string) {
		log := logging.MustGetLogger("dmsgtcp")
		if dmsgDisc == "" {
			log.Fatal("DMSG discovery URL not specified")
		}

		// Create the context and cancel function
		ctx, cancel := cmdutil.SignalContext(context.Background(), log)
		defer cancel()

		// Generate keys if not provided
		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}

		// Initialize the DMSG client
		dmsgC, closeDmsg, err := cli.StartDmsg(ctx, log, pk, sk, &http.Client{}, dmsgDisc, 1)
		if err != nil {
			log.WithError(err).Fatal("failed to start dmsg")
		}
		defer closeDmsg()

		go func() {
			<-ctx.Done()
			cancel()
			closeDmsg()
			os.Exit(0)
		}()

		// Listen on the specified DMSG port
		lis, err := dmsgC.Listen(uint16(dmsgPort))
		if err != nil {
			log.WithError(err).Fatal("Failed to listen on DMSG port")
		}
		defer lis.Close()

		log.Infof("Serving Hello World TCP on DMSG address %s", lis.Addr())

		// Handle system interrupt (Ctrl + C)
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

		// Start a goroutine to wait for shutdown signals
		go func() {
			<-signalChan
			log.Info("Received shutdown signal.")
			lis.Close()
			closeDmsg()
			cancel() // Cancel context to terminate DMSG client and server
		}()

		// Accept TCP connections and respond with "Hello, World!"
		for {
			conn, err := lis.Accept()
			if err != nil {
				// If the server was closed or a signal was received, break out
				if ctx.Err() != nil {
					log.Info("Server shutting down...")
					return
				}
				log.WithError(err).Error("Failed to accept connection")
				continue
			}
			go handleConnection(conn, log)
		}
	},
}

func handleConnection(conn net.Conn, log *logging.Logger) {
	defer conn.Close()
	log.Infof("Received connection from %s", conn.RemoteAddr())

	// Write "Hello, World!" message to the connection
	_, err := conn.Write([]byte("Hello, World!\n"))
	if err != nil {
		log.WithError(err).Error("Failed to write response")
	}
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
	RootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for dmsgpty-cli")
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
