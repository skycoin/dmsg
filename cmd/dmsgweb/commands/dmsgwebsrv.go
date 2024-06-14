// Package commands cmd/dmsgweb/commands/dmsgweb.go
package commands

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/bitfield/script"
	"github.com/gin-gonic/gin"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

const dmsgwebsrvenvname = "DMSGWEBSRV"

var dmsgwebsrvconffile = os.Getenv(dmsgwebsrvenvname)

func init() {
	RootCmd.AddCommand(srvCmd)
	srvCmd.Flags().UintSliceVarP(&localPort, "lport", "l", scriptExecUintSlice("${LOCALPORT[@]:-8086}", dmsgwebsrvconffile), "local application http interface port(s)")
	srvCmd.Flags().UintSliceVarP(&dmsgPort, "dport", "d", scriptExecUintSlice("${DMSGPORT[@]:-80}", dmsgwebsrvconffile), "dmsg port(s) to serve")
	srvCmd.Flags().StringVarP(&wl, "wl", "w", scriptExecArray("${WHITELISTPKS[@]}", dmsgwebsrvconffile), "whitelisted keys for dmsg authenticated routes\r")
	srvCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", skyenv.DmsgDiscAddr, "dmsg discovery url")
	srvCmd.Flags().IntVarP(&dmsgSess, "dsess", "e", scriptExecInt("${DMSGSESSIONS:-1}", dmsgwebsrvconffile), "dmsg sessions")
	srvCmd.Flags().BoolVarP(&rawTCP, "rt", "c", false, "proxy local port as raw TCP") // New flag
	srvCmd.Flags().BoolVarP(&rawUDP, "ru", "u", false, "proxy local port as raw UDP") // New flag

	if os.Getenv("DMSGWEBSRV_SK") != "" {
		sk.Set(os.Getenv("DMSGWEBSRV_SK")) //nolint
	}
	if scriptExecString("${DMSGWEBSRV_SK}", dmsgwebsrvconffile) != "" {
		sk.Set(scriptExecString("${DMSGWEBSRV_SK}", dmsgwebsrvconffile)) //nolint
	}
	pk, _ = sk.PubKey() //nolint
	srvCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")
	srvCmd.Flags().BoolVarP(&isEnvs, "envs", "z", false, "show example .conf file")

	srvCmd.CompletionOptions.DisableDefaultCmd = true
}

var srvCmd = &cobra.Command{
	Use:   "srv",
	Short: "serve http or raw TCP from local port over dmsg",
	Long: `DMSG web server - serve http or raw TCP interface from local port over dmsg` + func() string {
		if _, err := os.Stat(dmsgwebsrvconffile); err == nil {
			return `
	dmsenv file detected: ` + dmsgwebsrvconffile
		}
		return `
	.conf file may also be specified with
	` + dmsgwebsrvenvname + `=/path/to/dmsgwebsrv.conf skywire dmsg web srv`
	}(),
	Run: func(_ *cobra.Command, _ []string) {
		if isEnvs {
			envfile := srvenvfileLinux
			if runtime.GOOS == "windows" {
				envfileslice, _ := script.Echo(envfile).Slice() //nolint
				for i := range envfileslice {
					efs, _ := script.Echo(envfileslice[i]).Reject("##").Reject("#-").Reject("# ").Replace("#", "#$").String() //nolint
					if efs != "" && efs != "\n" {
						envfileslice[i] = strings.ReplaceAll(efs, "\n", "")
					}
				}
				envfile = strings.Join(envfileslice, "\n")
			}
			fmt.Println(envfile)
			os.Exit(0)
		}

		server()
	},
}

func server() {
	log := logging.MustGetLogger("dmsgwebsrv")
	if len(localPort) != len(dmsgPort) {
		log.Fatal(fmt.Sprintf("the same number of local ports as dmsg ports must be specified ; local ports: %v ; dmsg ports: %v", len(localPort), len(dmsgPort)))
	}
	if rawTCP && rawUDP {
		log.Fatal("must specify either --rt or --ru flags not both")
	}

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)

	defer cancel()
	pk, err = sk.PubKey()
	if err != nil {
		pk, sk = cipher.GenerateKeyPair()
	}
	log.Infof("dmsg client pk: %v", pk.String())

	if wl != "" {
		wlk := strings.Split(wl, ",")
		for _, key := range wlk {
			var pk0 cipher.PubKey
			err := pk0.Set(key)
			if err == nil {
				wlkeys = append(wlkeys, pk0)
			}
		}
	}
	if len(wlkeys) > 0 {
		if len(wlkeys) == 1 {
			log.Info(fmt.Sprintf("%d key whitelisted", len(wlkeys)))
		} else {
			log.Info(fmt.Sprintf("%d keys whitelisted", len(wlkeys)))
		}
	}

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

	var listN []net.Listener

	for _, dport := range dmsgPort {
		lis, err := dmsgC.Listen(uint16(dport))
		if err != nil {
			log.Fatalf("Error listening on port %d: %v", dport, err)
		}

		listN = append(listN, lis)

		dport := dport
		go func(l net.Listener, port uint) {
			<-ctx.Done()
			if err := l.Close(); err != nil {
				log.Printf("Error closing listener on port %d: %v", port, err)
				log.WithError(err).Error()
			}
		}(lis, dport)
	}

	wg := new(sync.WaitGroup)

	for i, lpt := range localPort {
		wg.Add(1)
		go func(localPort uint, lis net.Listener) {
			defer wg.Done()
			if rawTCP {
				proxyTCPConnections(localPort, lis, log)
			}
			//			if rawUDP {
			//				handleUDPConnection(localPort, lis, log)
			//			}
			if !rawTCP && !rawUDP {
				proxyHTTPConnections(localPort, lis, log)
			}
		}(lpt, listN[i])
	}

	wg.Wait()
}

func proxyHTTPConnections(localPort uint, lis net.Listener, log *logging.Logger) {
	r1 := gin.New()
	r1.Use(gin.Recovery())
	r1.Use(loggingMiddleware())

	authRoute := r1.Group("/")
	if len(wlkeys) > 0 {
		authRoute.Use(whitelistAuth(wlkeys))
	}
	authRoute.Any("/*path", func(c *gin.Context) {
		targetURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%v%s?%s", localPort, c.Request.URL.Path, c.Request.URL.RawQuery)) //nolint
		proxy := httputil.ReverseProxy{
			Director: func(req *http.Request) {
				req.URL = targetURL
				req.Host = targetURL.Host
				req.Method = c.Request.Method
			},
			Transport: &http.Transport{},
		}
		proxy.ServeHTTP(c.Writer, c.Request)
	})
	serve := &http.Server{
		Handler:           &ginHandler{Router: r1},
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
	}
	log.Printf("Serving HTTP on dmsg port %v with DMSG listener %s", localPort, lis.Addr().String())
	if err := serve.Serve(lis); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Serve: %v", err)
	}
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

/*
func handleUDPConnection(localPort uint, conn net.PacketConn, dmsgC *dmsg.Client, log *logging.Logger) {
	buffer := make([]byte, 65535)

	for {
		n, addr, err := conn.ReadFrom(buffer)
		if err != nil {
			log.Printf("Error reading UDP packet: %v", err)
			continue
		}

		err = dmsgC.SendUDP(buffer[:n], localPort, addr)
		if err != nil {
			log.Printf("Error sending UDP packet via dmsg client: %v", err)
			continue
		}

		responseBuffer := make([]byte, 65535)
		n, _, err = dmsgC.ReceiveUDP(responseBuffer)
		if err != nil {
			log.Printf("Error receiving UDP response from dmsg client: %v", err)
			continue
		}

		_, err = conn.WriteTo(responseBuffer[:n], addr)
		if err != nil {
			log.Printf("Error sending UDP response to client: %v", err)
			continue
		}
	}
}

func proxyUDPConnections(conn net.PacketConn, data []byte, addr net.Addr, webPort uint, log *logging.Logger) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			return
		}

		go handleUDPConnection(conn, localPort, log)
	}
}
*/

const srvenvfileLinux = `
#########################################################################
#--	DMSGWEB SRV CONFIG TEMPLATE
#--		Defaults shown
#--		Uncomment to change default value
#########################################################################

#--	DMSG port to serve
#DMSGPORT=('80')

#--	Local Port to serve over dmsg
LOCALPORT=('8086')

#--	Number of dmsg servers to connect to (0 unlimits)
#DMSGSESSIONS=1

#--	Set secret key
#DMSGWEBSRV_SK=''

#--	Whitelisted keys to access the web interface
#WHITELISTPKS=('')

#-- Proxy as raw TCP
#RAW_TCP=false
`
