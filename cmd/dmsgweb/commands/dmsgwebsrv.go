// Package commands cmd/dmsgweb/commands/dmsgweb.go
package commands

import (
	"context"
	"fmt"
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
	srvCmd.Flags().UintSliceVarP(&localPort, "lport", "l", scriptExecUintSlice("${LOCALPORT[@]:-8086}", dmsgwebsrvconffile), "local application http interface port")
	srvCmd.Flags().UintSliceVarP(&dmsgPort, "dport", "d", scriptExecUintSlice("${DMSGPORT[@]:-80}", dmsgwebsrvconffile), "dmsg port to serve")
	srvCmd.Flags().StringVarP(&wl, "wl", "w", scriptExecArray("${WHITELISTPKS[@]}", dmsgwebsrvconffile), "whitelisted keys for dmsg authenticated routes\r")
	srvCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", skyenv.DmsgDiscAddr, "dmsg discovery url")
	srvCmd.Flags().IntVarP(&dmsgSess, "dsess", "e", scriptExecInt("${DMSGSESSIONS:-1}", dmsgwebsrvconffile), "dmsg sessions")
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
	Short: "serve http from local port over dmsg",
	Long: `DMSG web server - serve http interface from local port over dmsg` + func() string {
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

	ctx, cancel := cmdutil.SignalContext(context.Background(), log)

	defer cancel()
	pk, err = sk.PubKey()
	if err != nil {
		pk, sk = cipher.GenerateKeyPair()
	}
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

		go func(l net.Listener) {
			<-ctx.Done()
			if err := l.Close(); err != nil {
				log.Printf("Error closing listener on port %d: %v", dport, err)
				log.WithError(err).Error()
			}
		}(lis)
	}

		wg := new(sync.WaitGroup)

		for i, lpt := range localPort {
			wg.Add(1)
			go func(localPort uint, lis net.Listener) {
				defer wg.Done()
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
				log.Printf("Serving on local port %v with DMSG listener %s", localPort, lis.Addr().String())
				if err := serve.Serve(lis); err != nil && err != http.ErrServerClosed {
					log.Fatalf("Serve: %v", err)
				}
			}(lpt, listN[i])
		}

		wg.Wait()
}

const srvenvfileLinux = `
#########################################################################
#--	DMSGWEB SRV CONFIG TEMPLATE
#--		Defaults shown
#--		Uncomment to change default value
#########################################################################

#--	DMSG port to serve
#DMSGPORT=80

#--	Local Port to serve over dmsg
LOCALPORT=8086

#--	Number of dmsg servers to connect to (0 unlimits)
#DMSGSESSIONS=1

#--	Set secret key
#DMSGWEBSRV_SK=''

#--	Whitelisted keys to access the web interface
#WHITELISTPKS=('')
`
