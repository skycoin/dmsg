package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	cc "github.com/ivanpirog/coloredcobra"
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
	httpC              http.Client
	dmsgC              *dmsg.Client
	closeDmsg          func()
	dmsgDisc           string
	dmsgSessions       int
	dmsgAddr           []string
	dialPK             []cipher.PubKey
	filterDomainSuffix string
	sk                 cipher.SecKey
	pk                 cipher.PubKey
	dmsgWebLog         *logging.Logger
	logLvl             string
	webPort            []uint
	proxyPort          uint
	addProxy           string
	resolveDmsgAddr    []string
	wg                 sync.WaitGroup
	isEnvs             bool
	dmsgPort           uint
	dmsgPorts          []uint
	dmsgSess           int
	wl                 []string
	wlkeys             []cipher.PubKey
	localPort          uint
	err                error
	rawTCP             []bool
	RootCmd = srvCmd
)


func init() {
	srvCmd.Flags().UintVarP(&localPort, "lport", "l", 8086, "local application http interface port(s)")
	srvCmd.Flags().UintVarP(&dmsgPort, "dport", "d", 8086, "dmsg port(s) to serve")
	srvCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", skyenv.DmsgDiscAddr, "dmsg discovery url")
	srvCmd.Flags().IntVarP(&dmsgSess, "dsess", "e", 1, "dmsg sessions")
	srvCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")

	srvCmd.CompletionOptions.DisableDefaultCmd = true
	var helpflag bool
	srvCmd.SetUsageTemplate(help)
	srvCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for dmsgweb")
	srvCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	srvCmd.PersistentFlags().MarkHidden("help") //nolint
}
var srvCmd = &cobra.Command{
	Use:   "srv",
	Short: "serve raw TCP from local port over dmsg",
	Long: `DMSG web server - serve http or raw TCP interface from local port over dmsg`,
	Run: func(_ *cobra.Command, _ []string) {
		server()
	},
}

func server() {
	log := logging.MustGetLogger("dmsgwebsrv")

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)

	defer cancel()
	pk, err = sk.PubKey()
	if err != nil {
		pk, sk = cipher.GenerateKeyPair()
	}
	log.Infof("dmsg client pk: %v", pk.String())


	dmsgC := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
	defer func() {
		if err := dmsgC.Close(); err != nil {
			log.WithError(err).Error()
		}
	}()

	go dmsgC.Serve(context.Background())

	select {
	case <-ctx.Done():
		log.WithError(ctx.Err()).Warn()
		return

	case <-dmsgC.Ready():
	}

	lis, err := dmsgC.Listen(uint16(dmsgPort))
	if err != nil {
		log.Fatalf("Error listening on port %d: %v", dmsgPort, err)
	}


	go func(l net.Listener, port uint) {
		<-ctx.Done()
		if err := l.Close(); err != nil {
			log.Printf("Error closing listener on port %d: %v", port, err)
			log.WithError(err).Error()
		}
	}(lis, dmsgPort)


	wg := new(sync.WaitGroup)

	wg.Add(1)
	go func(localPort uint, lis net.Listener) {
		defer wg.Done()
		proxyTCPConnections(localPort, lis, log)
	}(localPort, lis)

	wg.Wait()
}

func proxyTCPConnections(localPort uint, lis net.Listener, log *logging.Logger) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			return
		}

		go handleTCPConnection(conn, localPort, log)
	}
}

func handleTCPConnection(dmsgConn net.Conn, localPort uint, log *logging.Logger) {
	defer dmsgConn.Close() //nolint

	localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
	if err != nil {
		log.Printf("Error connecting to local port %d: %v", localPort, err)
		return
	}
	defer localConn.Close() //nolint

	copyConn := func(dst net.Conn, src net.Conn) {
		_, err := io.Copy(dst, src)
		if err != nil {
			log.Printf("Error during copy: %v", err)
		}
	}

	go copyConn(dmsgConn, localConn)
	go copyConn(localConn, dmsgConn)
}

func startDmsg(ctx context.Context, pk cipher.PubKey, sk cipher.SecKey) (dmsgC *dmsg.Client, stop func(), err error) {
	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, dmsgWebLog), &dmsg.Config{MinSessions: dmsgSessions})
	go dmsgC.Serve(context.Background())

	stop = func() {
		err := dmsgC.Close()
		dmsgWebLog.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
	dmsgWebLog.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc).
		Debug("Connecting to dmsg network...")

	select {
	case <-ctx.Done():
		stop()
		os.Exit(0)
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsgWebLog.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}
