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
	"regexp"
	"strings"
	"syscall"

	"github.com/confiant-inc/go-socks5"
	"github.com/gin-gonic/gin"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"

	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

func init() {
	RootCmd.AddCommand(genKeysCmd)
	RootCmd.Flags().StringVarP(&filterDomainSuffix, "filter", "f", ".dmsg", "domain suffix to filter")
	RootCmd.Flags().StringVarP(&proxyPort, "socks", "q", "4445", "port to serve the socks5 proxy")
	RootCmd.Flags().StringVarP(&addProxy, "proxy", "r", "", "configure additional socks5 proxy for dmsgweb (i.e. 127.0.0.1:1080)")
	RootCmd.Flags().StringVarP(&webPort, "port", "p", "8080", "port to serve the web application")
	RootCmd.Flags().StringVarP(&resolveDmsgAddr, "resolve", "t", "", "resolve the specified dmsg address:port on the local port & disable proxy")
	RootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", dmsgDisc, "dmsg discovery url")
	RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to")
	RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "", "[ debug | warn | error | fatal | panic | trace | info ]")
	if os.Getenv("DMSGGET_SK") != "" {
		sk.Set(os.Getenv("DMSGGET_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified")
}

// RootCmd contains the root command for dmsgweb
var RootCmd = &cobra.Command{
	Use:   "web",
	Short: "DMSG resolving proxy & browser client",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐┬ ┬┌─┐┌┐
	 │││││└─┐│ ┬│││├┤ ├┴┐
	─┴┘┴ ┴└─┘└─┘└┴┘└─┘└─┘
  ` + "DMSG resolving proxy & browser client - access websites over dmsg",
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

		if filterDomainSuffix == "" {
			dmsgWebLog.Fatal("domain suffix to filter cannot be an empty string")
		}
		ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgWebLog)
		defer cancel()

		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
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
			os.Exit(0) //this should not be necessary
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
							port = webPort
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
			socksAddr := "127.0.0.1:" + proxyPort
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
		r := gin.New()

		r.Use(gin.Recovery())

		r.Use(loggingMiddleware())

		r.Any("/*path", func(c *gin.Context) {
			var urlStr string
			if resolveDmsgAddr != "" {
				urlStr = fmt.Sprintf("dmsg://%s%s", resolveDmsgAddr, c.Param("path"))
			} else {

				hostParts := strings.Split(c.Request.Host, ":")
				var dmsgp string
				if len(hostParts) > 1 {
					dmsgp = hostParts[1]
				} else {
					dmsgp = "80"
				}
				urlStr = fmt.Sprintf("dmsg://%s:%s%s", strings.TrimRight(hostParts[0], filterDomainSuffix), dmsgp, c.Param("path"))
			}

			req, err := http.NewRequest(http.MethodGet, urlStr, nil)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to create HTTP request")
				return
			}

			resp, err := httpC.Do(req)
			if err != nil {
				c.String(http.StatusInternalServerError, "Failed to connect to HTTP server")
				return
			}
			defer resp.Body.Close() //nolint

			c.Status(http.StatusOK)
			io.Copy(c.Writer, resp.Body) //nolint
		})
		wg.Add(1)
		go func() {
			dmsgWebLog.Debug("Serving http on " + webPort)
			r.Run(":" + webPort) //nolint
			dmsgWebLog.Debug("Stopped serving http on " + webPort)
			wg.Done()
		}()
		wg.Wait()
	},
}
