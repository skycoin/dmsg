// Package commands cmd/dmsgweb/commands/dmsgwebsrv.go
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

	"github.com/bitfield/script"
	"github.com/gin-gonic/gin"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

const dmsgwebsrvenvname = "DMSGWEBSRV"

func init() {
	RootCmd.AddCommand(srvCmd)
	srvCmd.Flags().UintSliceVarP(&localPort, "lport", "l", scriptExecUintSlice("${LOCALPORT[@]:-8086}", srvenvfileLinux), "local application HTTP interface port(s)")
	srvCmd.Flags().UintSliceVarP(&dmsgPort, "dport", "d", scriptExecUintSlice("${DMSGPORT[@]:-80}", srvenvfileLinux), "DMSG port(s) to serve")
	srvCmd.Flags().StringSliceVarP(&wl, "wl", "w", scriptExecStringSlice("${WHITELISTPKS[@]}", srvenvfileLinux), "whitelisted keys for DMSG authenticated routes")
	srvCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", dmsgDisc, "DMSG discovery URL(s)")
	srvCmd.Flags().StringVarP(&proxyAddr, "proxy", "x", proxyAddr, "connect to DMSG via proxy (e.g., '127.0.0.1:1080')")
	srvCmd.Flags().IntVarP(&dmsgSess, "dsess", "e", scriptExecInt("${DMSGSESSIONS:-1}", srvenvfileLinux), "DMSG sessions")
	srvCmd.Flags().BoolSliceVarP(&rawTCP, "rt", "c", scriptExecBoolSlice("${RAWTCP[@]:-false}", srvenvfileLinux), "proxy local port as raw TCP")
	srvCmd.Flags().BoolVarP(&isEnvs, "envs", "z", false, "show example .conf file")

	if os.Getenv("DMSGWEBSRVSK") != "" {
		sk.Set(os.Getenv("DMSGWEBSRVSK"))
	}
	if scriptExecString("${DMSGWEBSRVSK}", srvenvfileLinux) != "" {
		sk.Set(scriptExecString("${DMSGWEBSRVSK}", srvenvfileLinux))
	}
	pk, _ = sk.PubKey()
	srvCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified")

	srvCmd.CompletionOptions.DisableDefaultCmd = true
}

var srvCmd = &cobra.Command{
	Use:   "srv",
	Short: "Serve HTTP or raw TCP from local port over DMSG",
	Long: `DMSG web server - serve HTTP or raw TCP interface from local port over DMSG` + func() string {
		if _, err := os.Stat(srvenvfileLinux); err == nil {
			return "\n\t.dmsenv file detected: " + srvenvfileLinux
		}
		return "\n\t.conf file may also be specified with " + dmsgwebsrvenvname + `=/path/to/dmsgwebsrv.conf skywire dmsg web srv`
	}(),
	Run: func(_ *cobra.Command, _ []string) {
		if isEnvs {
			envfile := srvenvfileLinux
			if runtime.GOOS == "windows" {
				envfileslice, _ := script.Echo(envfile).Slice()
				for i := range envfileslice {
					efs, _ := script.Echo(envfileslice[i]).Reject("##").Reject("#-").Reject("# ").Replace("#", "#$").String()
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

	if len(localPort) != len(dmsgPort) || len(localPort) != len(rawTCP) {
		log.Fatal("The number of local ports, DMSG ports, and raw TCP flags must be the same")
	}

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)
	defer cancel()

	pk, err := sk.PubKey()
	if err != nil {
		pk, sk = cipher.GenerateKeyPair()
	}
	log.Infof("DMSG client public key: %v", pk.String())

	if len(wl) > 0 {
		for _, key := range wl {
			var pk cipher.PubKey
			if err := pk.Set(key); err == nil {
				wlkeys = append(wlkeys, pk)
			}
		}
		log.Infof("%d keys whitelisted", len(wlkeys))
	}

	if proxyAddr != "" {
		var err error
		dialer, err = proxy.SOCKS5("tcp", proxyAddr, nil, proxy.Direct)
		if err != nil {
			log.Fatalf("Error creating SOCKS5 dialer: %v", err)
		}
		httpClient = &http.Client{Transport: &http.Transport{Dial: dialer.Dial}}
	}

	dmsgClient := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, log), dmsg.DefaultConfig())
	defer func() {
		if err := dmsgClient.Close(); err != nil {
			log.WithError(err).Error()
		}
	}()
	go dmsgClient.Serve(context.Background())

	select {
	case <-ctx.Done():
		log.WithError(ctx.Err()).Warn()
		return
	case <-dmsgClient.Ready():
	}

	wg := sync.WaitGroup{}
	for i := range localPort {
		lis, err := dmsgClient.Listen(uint16(dmsgPort[i]))
		if err != nil {
			log.Fatalf("Error listening on DMSG port %d: %v", dmsgPort[i], err)
		}
		wg.Add(1)
		go func(localPort uint, rawTCP bool, listener net.Listener) {
			defer wg.Done()
			if rawTCP {
				proxyTCPConnections(localPort, listener, log)
			} else {
				proxyHTTPConnections(localPort, listener, log)
			}
		}(localPort[i], rawTCP[i], lis)
	}
	wg.Wait()
}

func proxyHTTPConnections(localPort uint, listener net.Listener, log *logging.Logger) {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(loggingMiddleware())

	authRoute := router.Group("/")
	if len(wlkeys) > 0 {
		authRoute.Use(whitelistAuth(wlkeys))
	}
	authRoute.Any("/*path", func(c *gin.Context) {
		targetURL := fmt.Sprintf("http://127.0.0.1:%d%s?%s", localPort, c.Request.URL.Path, c.Request.URL.RawQuery)
		proxy := httputil.ReverseProxy{Director: func(req *http.Request) {
			req.URL, _ = url.Parse(targetURL)
			req.Host = req.URL.Host
		}}
		proxy.ServeHTTP(c.Writer, c.Request)
	})

	server := &http.Server{Handler: router}
	if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
		log.Fatalf("HTTP server error: %v", err)
	}
}

func proxyTCPConnections(localPort uint, listener net.Listener, log *logging.Logger) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Errorf("Error accepting connection: %v", err)
			return
		}

		go func(dmsgConn net.Conn) {
			defer dmsgConn.Close()

			localConn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", localPort))
			if err != nil {
				log.Errorf("Error connecting to local port %d: %v", localPort, err)
				return
			}
			defer localConn.Close()

			go io.Copy(dmsgConn, localConn)
			io.Copy(localConn, dmsgConn)
		}(conn)
	}
}

const srvenvfileLinux = `
#########################################################################
#--	DMSGWEB SRV CONFIG TEMPLATE
#--		Defaults shown
#--		Uncomment to change default value
#--		LOCALPORT and DMSGPORT must contain the same number of elements
#########################################################################

#--	DMSG port to serve
#DMSGPORT=('80')

#--	Local Port to serve over dmsg
#LOCALPORT=('8086')

#--	Number of dmsg servers to connect to (0 unlimits)
#DMSGSESSIONS=1

#--	Set secret key
#DMSGWEBSRVSK=''

#--	Whitelisted keys to access the web interface
#WHITELISTPKS=('')

#-- Proxy as raw TCP
#RAWTCP=('false')
`
