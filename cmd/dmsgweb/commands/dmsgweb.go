// Package commands cmd/dmsgweb/commands/dmsgweb.go
package commands

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
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/bitfield/script"
	"github.com/confiant-inc/go-socks5"
	"github.com/gin-gonic/gin"
	"github.com/skycoin/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"

	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

type customResolver struct{}

func (r *customResolver) Resolve(ctx context.Context, name string) (context.Context, net.IP, error) {
	// Handle custom name resolution for .dmsg domains
	regexPattern := `\.` + filterDomainSuffix + `(:[0-9]+)?$`
	match, _ := regexp.MatchString(regexPattern, name) //nolint:errcheck
	if match {
		ip := net.ParseIP("127.0.0.1")
		if ip == nil {
			return ctx, nil, fmt.Errorf("failed to parse IP address")
		}
		// Modify the context to include the desired port
		ctx = context.WithValue(ctx, "port", fmt.Sprintf("%v", webPort)) //nolint
		return ctx, ip, nil
	}
	// Use default name resolution for other domains
	return ctx, nil, nil
}

const dmsgwebenvname = "DMSGWEB"

var dmsgwebconffile = os.Getenv(dmsgwebenvname)

func init() {

	RootCmd.Flags().StringVarP(&filterDomainSuffix, "filter", "f", ".dmsg", "domain suffix to filter")
	RootCmd.Flags().UintVarP(&proxyPort, "socks", "q", scriptExecUint("${PROXYPORT:-4445}", dmsgwebconffile), "port to serve the socks5 proxy")
	RootCmd.Flags().StringVarP(&addProxy, "proxy", "r", scriptExecString("${ADDPROXY}", dmsgwebconffile), "configure additional socks5 proxy for dmsgweb (i.e. 127.0.0.1:1080)")
	RootCmd.Flags().UintVarP(&webPort, "port", "p", scriptExecUint("${WEBPORT:-8080}", dmsgwebconffile), "port to serve the web application")
	RootCmd.Flags().StringVarP(&resolveDmsgAddr, "resolve", "t", scriptExecString("${RESOLVEPK}", dmsgwebconffile), "resolve the specified dmsg address:port on the local port & disable proxy")
	RootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "d", skyenv.DmsgDiscAddr, "dmsg discovery url")
	RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", scriptExecInt("${DMSGSESSIONS:-1}", dmsgwebconffile), "number of dmsg servers to connect to")
	RootCmd.Flags().BoolVarP(&rawTCP, "rt", "c", false, "proxy local port as raw TCP") // New flag
	RootCmd.Flags().BoolVarP(&rawUDP, "ru", "u", false, "proxy local port as raw UDP") // New flag

	RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m")
	if os.Getenv("DMSGWEB_SK") != "" {
		sk.Set(os.Getenv("DMSGWEB_SK")) //nolint
	}
	if scriptExecString("${DMSGWEB_SK}", dmsgwebconffile) != "" {
		sk.Set(scriptExecString("${DMSGWEB_SK}", dmsgwebconffile)) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")
	RootCmd.Flags().BoolVarP(&isEnvs, "envs", "z", false, "show example .conf file")

}

// RootCmd contains the root command for dmsgweb
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Short: "DMSG resolving proxy & browser client",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐┬ ┬┌─┐┌┐
	 │││││└─┐│ ┬│││├┤ ├┴┐
	─┴┘┴ ┴└─┘└─┘└┴┘└─┘└─┘
DMSG resolving proxy & browser client - access websites and http interfaces over dmsg` + func() string {
		if _, err := os.Stat(dmsgwebconffile); err == nil {
			return `
dmsgweb conf file detected: ` + dmsgwebconffile
		}
		return `
.conf file may also be specified with
` + dmsgwebenvname + `=/path/to/dmsgweb.conf skywire dmsg web`
	}(),
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	Run: func(cmd *cobra.Command, _ []string) {
		if isEnvs {
			envfile := envfileLinux
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
		if rawTCP && rawUDP {
			log.Fatal("must specify either --rt or --ru flags not both")
		}

		if filterDomainSuffix == "" {
			dmsgWebLog.Fatal("domain suffix to filter cannot be an empty string")
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
		dmsgWebLog.Info("dmsg client pk: %v", pk.String())
		dmsgAddr = strings.Split(resolveDmsgAddr, ":")
		err = dialPK.Set(dmsgAddr[0])
		if err != nil {
			log.Fatalf("failed to parse dmsg <address>:<port> : %v", err)
		}
		dmsgC, closeDmsg, err := startDmsg(ctx, pk, sk)
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

		httpC = http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}

		if resolveDmsgAddr == "" {
			// Create a SOCKS5 server with custom name resolution
			conf := &socks5.Config{
				Resolver: &customResolver{},
				Dial: func(ctx context.Context, network, addr string) (net.Conn, error) {
					host, _, err := net.SplitHostPort(addr)
					if err != nil {
						return nil, err
					}
					regexPattern := `\` + filterDomainSuffix + `(:[0-9]+)?$`
					match, _ := regexp.MatchString(regexPattern, host) //nolint:errcheck
					if match {
						port, ok := ctx.Value("port").(string)
						if !ok {
							port = fmt.Sprintf("%v", webPort)
						}
						addr = "localhost:" + port
					} else {
						if addProxy != "" {
							// Fallback to another SOCKS5 proxy
							dialer, err := proxy.SOCKS5("tcp", addProxy, nil, proxy.Direct)
							if err != nil {
								return nil, err
							}
							return dialer.Dial(network, addr)
						}
					}
					dmsgWebLog.Debug("Dialing address:", addr)
					return net.Dial(network, addr)
				},
			}

			// Start the SOCKS5 server
			socksAddr := fmt.Sprintf("127.0.0.1:%v", proxyPort)
			log.Printf("SOCKS5 proxy server started on %s", socksAddr)

			server, err := socks5.New(conf)
			if err != nil {
				log.Fatalf("Failed to create SOCKS5 server: %v", err)
			}

			wg.Add(1)
			go func() {
				dmsgWebLog.Debug("Serving SOCKS5 proxy on " + socksAddr)
				err := server.ListenAndServe("tcp", socksAddr)
				if err != nil {
					log.Fatalf("Failed to start SOCKS5 server: %v", err)
				}
				defer server.Close()
				dmsgWebLog.Debug("Stopped serving SOCKS5 proxy on " + socksAddr)
			}()
		}

		if rawTCP {
			proxyTCPConn()
		}
		if rawUDP {
			log.Fatalf("handling raw udp not yet implemented")
//			proxyUDPConn()
		}
		if !rawTCP && !rawUDP {
			proxyHTTPConn()
		}
		wg.Wait()
	},
}

func proxyHTTPConn() {
	r := gin.New()

	r.Use(gin.Recovery())

	r.Use(loggingMiddleware())

	r.Any("/*path", func(c *gin.Context) {
		var urlStr string
		if resolveDmsgAddr != "" {
			urlStr = fmt.Sprintf("dmsg://%s%s", resolveDmsgAddr, c.Param("path"))
			if c.Request.URL.RawQuery != "" {
				urlStr = fmt.Sprintf("%s?%s", urlStr, c.Request.URL.RawQuery)
			}
		} else {
			hostParts := strings.Split(c.Request.Host, ":")
			var dmsgp string
			if len(hostParts) > 1 {
				dmsgp = hostParts[1]
			} else {
				dmsgp = "80"
			}
			urlStr = fmt.Sprintf("dmsg://%s:%s%s", strings.TrimRight(hostParts[0], filterDomainSuffix), dmsgp, c.Param("path"))
			if c.Request.URL.RawQuery != "" {
				urlStr = fmt.Sprintf("%s?%s", urlStr, c.Request.URL.RawQuery)
			}
		}

		fmt.Printf("Proxying request: %s %s\n", c.Request.Method, urlStr)
		req, err := http.NewRequest(c.Request.Method, urlStr, c.Request.Body)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to create HTTP request")
			return
		}

		for header, values := range c.Request.Header {
			for _, value := range values {
				req.Header.Add(header, value)
			}
		}

		resp, err := httpC.Do(req)
		if err != nil {
			c.String(http.StatusInternalServerError, "Failed to connect to HTTP server")
			fmt.Printf("Error: %v\n", err)
			return
		}
		defer resp.Body.Close() //nolint

		for header, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(header, value)
			}
		}

		c.Status(resp.StatusCode)
		if _, err := io.Copy(c.Writer, resp.Body); err != nil {
			c.String(http.StatusInternalServerError, "Failed to copy response body")
			fmt.Printf("Error copying response body: %v\n", err)
		}
	})
	wg.Add(1)
	go func() {
		dmsgWebLog.Debug(fmt.Sprintf("Serving http on: http://127.0.0.1:%v", webPort))
		r.Run(":" + fmt.Sprintf("%v", webPort)) //nolint
		dmsgWebLog.Debug(fmt.Sprintf("Stopped serving http on: http://127.0.0.1:%v", webPort))
		wg.Done()
	}()
}
func proxyTCPConn() {
	// Start listening on the specified local port
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", webPort))
	if err != nil {
		log.Fatalf("Failed to start TCP listener on port %d: %v", webPort, err)
	}
	defer listener.Close() //nolint
	log.Printf("Serving TCP on 127.0.0.1:%d", webPort)

	// Accept incoming TCP connections and proxy them
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

			var dialPort uint64
			if len(dmsgAddr) > 1 {
				dialPort, err = strconv.ParseUint(dmsgAddr[1], 10, 64)
				if err != nil {
					log.Fatalf("Failed to parse dmsg port: %v", err)
					return
				}
			} else {
				dialPort = uint64(80)
			}

			// Dial the dmsg address using the dmsg client
			dmsgConn, err := dmsgC.DialStream(context.Background(), dmsg.Addr{PK: dialPK, Port: uint16(dialPort)})
			if err != nil {
				log.Printf("Failed to dial dmsg address %s: %v", resolveDmsgAddr, err)
				return
			}
			defer dmsgConn.Close() //nolint

			// Proxy data from TCP connection to dmsg connection
			go func() {
				_, err := io.Copy(dmsgConn, conn)
				if err != nil {
					log.Printf("Error copying data to dmsg server: %v", err)
				}
				// Close dmsgConn after copying is done
				dmsgConn.Close() //nolint
			}()

			// Proxy data from dmsg connection to TCP connection
			go func() {
				_, err := io.Copy(conn, dmsgConn)
				if err != nil {
					log.Printf("Error copying data from dmsg server: %v", err)
				}
				// Close conn after copying is done
				conn.Close() //nolint
			}()
		}(conn)
	}
}

/*
	func proxyUDPConn(webPort uint, dmsgC *dmsg.Client, log *logging.Logger) {
		// Resolve the dmsg address
		var dialPort uint64
		if len(dmsgAddr) > 1 {
			dialPort, _ = strconv.ParseUint(dmsgAddr[1], 10, 64)
		} else {
			dialPort = 80
		}

		// Listen for incoming UDP packets on the specified port
		addr := fmt.Sprintf(":%d", webPort)
		conn, err := net.ListenPacket("udp", addr)
		if err != nil {
			log.Fatalf("Failed to listen on UDP port %d: %v", webPort, err)
		}
		defer conn.Close()
		log.Printf("Serving UDP on 127.0.0.1:%d", webPort)

		// Buffer to hold incoming UDP data
		buffer := make([]byte, 65535) // Maximum UDP packet size

		for {
			// Read UDP packet from the connection
			n, addr, err := conn.ReadFrom(buffer)
			if err != nil {
				log.Printf("Error reading UDP packet: %v", err)
				continue
			}

			// Destination dmsg address
			dmsgAddr := dmsg.Addr{PK: dialPK, Port: uint16(dialPort)}

			// Dial dmsg connection
			dmsgConn, err := dmsgC.DialPacketConn(context.Background(), dmsgAddr)
			if err != nil {
				log.Printf("Failed to dial dmsg address %s: %v", dmsgAddr.String(), err)
				continue
			}

			// Write UDP packet to dmsg connection
			_, err = dmsgConn.WriteTo(buffer[:n], addr)
			if err != nil {
				log.Printf("Error writing UDP packet to dmsg server: %v", err)
			}

			// Close dmsg connection
			dmsgConn.Close()
		}
	}
*/
const envfileLinux = `
#########################################################################
#--	DMSGWEB CONFIG TEMPLATE
#--		Defaults shown
#--		Uncomment to change default value
#########################################################################

#--	Set port for proxy interface
#PROXYPORT=4445

#--	Configure additional proxy for dmsgvlc to use
#ADDPROXY='127.0.0.1:1080'

#--	Web Interface Port
#WEBPORT=8080

#--	Resove a specific PK to the web port (also disables proxy)
#RESOLVEPK=''

#--	Number of dmsg servers to connect to (0 unlimits)
#DMSGSESSIONS=1

#--	Dmsg port to use
#DMSGPORT=80

#--	Set secret key
#DMSGWEB_SK=''
`
