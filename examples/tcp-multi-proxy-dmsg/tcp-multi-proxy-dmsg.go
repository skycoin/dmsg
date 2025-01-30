package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

func main() {
	cc.Init(&cc.Config{
		RootCmd:         srvCmd,
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
	srvCmd.Execute()
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

var (
	localPorts []uint
	dmsgPorts  []uint
	dmsgDisc   string
	dmsgSess   int
	sk         cipher.SecKey
)

func init() {
	srvCmd.Flags().UintSliceVarP(&localPorts, "lport", "l", nil, "local application HTTP interface port(s) (comma-separated)")
	srvCmd.Flags().UintSliceVarP(&dmsgPorts, "dport", "d", nil, "DMSG port(s) to serve (comma-separated)")
	srvCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", dmsg.DiscAddr(false), "DMSG discovery URL")
	srvCmd.Flags().IntVarP(&dmsgSess, "dsess", "e", 1, "DMSG sessions")
	srvCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")

	srvCmd.CompletionOptions.DisableDefaultCmd = true
	var helpFlag bool
	srvCmd.SetUsageTemplate(help)
	srvCmd.PersistentFlags().BoolVarP(&helpFlag, "help", "h", false, "help for dmsgweb")
	srvCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	srvCmd.PersistentFlags().MarkHidden("help") //nolint
}

var srvCmd = &cobra.Command{
	Use:   "srv",
	Short: "Serve raw TCP from local ports over DMSG",
	Long:  `DMSG web server - serve HTTP or raw TCP interface from local ports over DMSG`,
	Run: func(_ *cobra.Command, _ []string) {
		server()
	},
}

func server() {
	log := logging.MustGetLogger("dmsgwebsrv")
	ctx, cancel := cmdutil.SignalContext(context.Background(), log)
	defer cancel()

	if len(localPorts) != len(dmsgPorts) {
		log.Fatalf("The number of local ports (%d) must match the number of DMSG ports (%d).", len(localPorts), len(dmsgPorts))
	}

	pk, err := sk.PubKey()
	if err != nil {
		pk, sk = cipher.GenerateKeyPair()
	}
	log.Infof("DMSG client public key: %v", pk.String())

	dmsgC := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
	defer func() {
		if err := dmsgC.Close(); err != nil {
			log.WithError(err).Error("Error closing DMSG client")
		}
	}()
	go dmsgC.Serve(ctx)

	select {
	case <-ctx.Done():
		log.WithError(ctx.Err()).Warn()
		return
	case <-dmsgC.Ready():
		log.Info("DMSG client is ready.")
	}

	wg := new(sync.WaitGroup)
	for i, localPort := range localPorts {
		dmsgPort := dmsgPorts[i]
		wg.Add(1)

		go func(localPort, dmsgPort uint) {
			defer wg.Done()
			proxyPort(ctx, dmsgC, localPort, dmsgPort, log)
		}(localPort, dmsgPort)
	}

	wg.Wait()
}

func proxyPort(ctx context.Context, dmsgC *dmsg.Client, localPort, dmsgPort uint, log *logging.Logger) {
	listener, err := dmsgC.Listen(uint16(dmsgPort))
	if err != nil {
		log.Fatalf("Error listening on DMSG port %d: %v", dmsgPort, err)
	}
	defer listener.Close()

	log.Infof("Started proxying local port %d to DMSG port %d", localPort, dmsgPort)

	go func() {
		<-ctx.Done()
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection on DMSG port %d: %v", dmsgPort, err)
			return
		}

		go handleTCPConnection(conn, localPort, log)
	}
}

func handleTCPConnection(dmsgConn net.Conn, localPort uint, log *logging.Logger) {
	defer dmsgConn.Close()

	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		log.Printf("Failed to connect to local port %d: %v", localPort, err)
		return
	}
	defer localConn.Close()

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		io.Copy(dmsgConn, localConn)
	}()

	go func() {
		defer wg.Done()
		io.Copy(localConn, dmsgConn)
	}()

	wg.Wait()
	log.Printf("Closed connection between local port %d and DMSG connection", localPort)
}
