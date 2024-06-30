package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"sync"

	"github.com/skycoin/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"
	cc "github.com/ivanpirog/coloredcobra"

	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/disc"


)



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
	RootCmd.Execute()
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
		dialPK             cipher.PubKey
		sk                 cipher.SecKey
		pk                 cipher.PubKey
		dmsgWebLog         *logging.Logger
		logLvl             string
		webPort            uint
		resolveDmsgAddr    string
		wg                 sync.WaitGroup
		dmsgPort           uint
		dmsgSess           int
		err                error
	)

	func init() {
		RootCmd.Flags().UintVarP(&webPort, "port", "p", 8080, "port to serve the web application")
		RootCmd.Flags().StringVarP(&resolveDmsgAddr, "resolve", "t", "", "resolve the specified dmsg address:port on the local port & disable proxy")
		RootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "d", skyenv.DmsgDiscAddr, "dmsg discovery url")
		RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to")
		RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m")
		RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")
	}

	// RootCmd contains the root command for dmsgweb
	var RootCmd = &cobra.Command{
		Use: func() string {
			return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
		}(),
		Short: "DMSG reverse tcp proxy",
		Long: "DMSG reverse tcp proxy",
		SilenceErrors:         true,
		SilenceUsage:          true,
		DisableSuggestions:    true,
		DisableFlagsInUseLine: true,
		Version:               buildinfo.Version(),
		Run: func(cmd *cobra.Command, _ []string) {

			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM) //nolint
			go func() {
				<-c
				os.Exit(1)
			}()
			if dmsgWebLog == nil {
				dmsgWebLog = logging.MustGetLogger("dmsgweb")
			}
			if logLvl != "" {
				if lvl, err := logging.LevelFromString(logLvl); err == nil {
					logging.SetLevel(lvl)
				}
			}

			if dmsgDisc == "" {
				dmsgDisc = skyenv.DmsgDiscAddr
			}
			ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgWebLog)
			defer cancel()

			pk, err := sk.PubKey()
			if err != nil {
				pk, sk = cipher.GenerateKeyPair()
			}
			dmsgWebLog.Info("dmsg client pk: ", pk.String())

			dmsgWebLog.Info("dmsg address to dial: ", resolveDmsgAddr)
			dmsgAddr = strings.Split(resolveDmsgAddr, ":")
			var setpk cipher.PubKey
			err = setpk.Set(dmsgAddr[0])
			if err != nil {
				log.Fatalf("failed to parse dmsg <address>:<port> : %v", err)
			}
			dialPK = setpk
			if len(dmsgAddr) > 1 {
				dport, err := strconv.ParseUint(dmsgAddr[1], 10, 64)
				if err != nil {
					log.Fatalf("Failed to parse dmsg port: %v", err)
				}
				dmsgPort = uint(dport)
			} else {
				dmsgPort = uint(80)
			}

			dmsgC, closeDmsg, err = startDmsg(ctx, pk, sk)
			if err != nil {
				dmsgWebLog.WithError(err).Fatal("failed to start dmsg")
			}
			defer closeDmsg()

			go func() {
				<-ctx.Done()
				cancel()
				closeDmsg()
				os.Exit(0)
			}()

			proxyTCPConn()
			wg.Wait()
		},
	}

	func proxyTCPConn() {
		listener, err := net.Listen("tcp", fmt.Sprintf(":%v", webPort))
		if err != nil {
			dmsgWebLog.Fatalf("Failed to start TCP listener on port %v: %v", webPort, err)
		}
		defer listener.Close() //nolint
		log.Printf("Serving TCP on 127.0.0.1:%v", webPort)

		for {
			conn, err := listener.Accept()
			if err != nil {
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			wg.Add(1)
			go func(conn net.Conn) {
				defer wg.Done()
				defer conn.Close() //nolint

				dmsgConn, err := dmsgC.DialStream(context.Background(), dmsg.Addr{PK: dialPK, Port: uint16(dmsgPort)})
				if err != nil {
					log.Printf("Failed to dial dmsg address %v:%v %v", dialPK.String(), dmsgPort, err)
					return
				}
				defer dmsgConn.Close() //nolint

				go func() {
					_, err := io.Copy(dmsgConn, conn)
					if err != nil {
						log.Printf("Error copying data to dmsg client: %v", err)
					}
					dmsgConn.Close() //nolint
				}()

				go func() {
					_, err := io.Copy(conn, dmsgConn)
					if err != nil {
						log.Printf("Error copying data from dmsg client: %v", err)
					}
					conn.Close() //nolint
				}()
			}(conn)
		}
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
