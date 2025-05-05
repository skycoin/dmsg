// Package commands cmd/dmsghttp/commands/dmsghttp.go
package commands

import (
	"context"
	"fmt"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/0magnet/calvin"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
)

var (
	sk           cipher.SecKey
	dmsgDisc     = dmsg.DiscAddr(false)
	serveDir     string
	dmsgPort     uint
	wl           string
	wlkeys       []cipher.PubKey
	useHTTP      bool
	logLvl       string
	proxyAddr    []string
	dmsgHTTPPath string
	dmsgSessions int
	dlog         = logging.MustGetLogger("dmsghttp")
)

func init() {
	RootCmd.Flags().SortFlags = false
	RootCmd.Flags().BoolVarP(&useHTTP, "http", "z", false, "use regular http to connect to dmsg discovery")
	//RootCmd.Flags().StringSliceVarP(&dmsgDiscs, "dmsg-disc", "c", []string{dmsg.DiscAddr(false)}, "dmsg discovery url(s)\033[0m\n\r")
	RootCmd.Flags().StringVarP(&dmsgHTTPPath, "dmsgconf", "F", "", "dmsghttp-config path")
	RootCmd.Flags().StringSliceVarP(&proxyAddr, "proxy", "p", proxyAddr, "connect to dmsg via proxy (i.e. '127.0.0.1:1080')")
	RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to\033[0m\n\r")
	RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "fatal", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m\n\r")
	RootCmd.Flags().StringVarP(&serveDir, "dir", "r", ".", "local dir to serve via dmsghttp")
	RootCmd.Flags().UintVarP(&dmsgPort, "port", "d", 80, "dmsg port to serve from")
	RootCmd.Flags().StringVarP(&wl, "wl", "w", "", "whitelist keys, comma separated")
	RootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "D", dmsgDisc, "dmsg discovery url")
	if os.Getenv("DMSGHTTP_SK") != "" {
		sk.Set(os.Getenv("DMSGHTTP_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\033[0m\n\r")

}

// RootCmd contains the root dmsghttp command
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Short: "DMSG http file server",
	Long: calvin.AsciiFont("dmsghttp") + `
	DMSG http file server`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),

	Run: func(_ *cobra.Command, _ []string) {
		if logLvl != "" {
			if lvl, err := logging.LevelFromString(logLvl); err == nil {
				logging.SetLevel(lvl)
			}
		}
		ctx, cancel := cmdutil.SignalContext(context.Background(), dlog)
		defer cancel()
		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}
		if wl != "" {
			wlk := strings.Split(wl, ",")
			for _, key := range wlk {
				var pubKey cipher.PubKey
				err := pubKey.Set(key)
				if err == nil {
					wlkeys = append(wlkeys, pubKey)
				}
			}
		}
		if len(wlkeys) > 0 {
			if len(wlkeys) == 1 {
				dlog.Info(fmt.Sprintf("%d key whitelisted", len(wlkeys)))
			} else {
				dlog.Info(fmt.Sprintf("%d keys whitelisted", len(wlkeys)))
			}
		}

		c := dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, dlog), dmsg.DefaultConfig())
		defer func() {
			if err := c.Close(); err != nil {
				dlog.WithError(err).Error()
			}
		}()

		go c.Serve(context.Background())

		select {
		case <-ctx.Done():
			dlog.WithError(ctx.Err()).Warn()
			return

		case <-c.Ready():
		}

		lis, err := c.Listen(uint16(dmsgPort))
		if err != nil {
			dlog.WithError(err).Fatal()
		}
		go func() {
			<-ctx.Done()
			if err := lis.Close(); err != nil {
				dlog.WithError(err).Error()
			}
		}()

		dlog.WithField("dir", serveDir).
			WithField("dmsg_addr", lis.Addr().String()).
			Info("Serving...")

		http.HandleFunc("/", fileServerHandler)
		serve := &http.Server{
			ReadHeaderTimeout: 3 * time.Second,
		}
		dlog.Fatal(serve.Serve(lis))

	},
}

func fileServerHandler(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Get the remote PK.
	remotePK, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	// Check if the remote PK is whitelisted.
	whitelisted := false
	if len(wlkeys) == 0 {
		whitelisted = true
	} else {
		for _, pubKey := range wlkeys {
			if remotePK == pubKey.String() {
				whitelisted = true
				break
			}
		}
	}

	// If the remote PK is whitelisted, serve the file.
	if whitelisted {
		filePath := serveDir + r.URL.Path
		file, err := os.Open(filePath) //nolint
		if err != nil {
			http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
			return
		}
		defer file.Close() //nolint

		_, filename := path.Split(filePath)
		w.Header().Set("Content-Type", mime.TypeByExtension(filepath.Ext(filename)))
		http.ServeContent(w, r, filename, time.Time{}, file)

		// Log the response status and time taken.
		elapsed := time.Since(start)
		dlog.Printf("[DMSGHTTP] %s %s | %d | %v | %s | %s %s\n", start.Format("2006/01/02 - 15:04:05"), r.RemoteAddr, http.StatusOK, elapsed, r.Method, r.Proto, r.URL)
		return
	}

	// Otherwise, return a 403 Forbidden error.
	http.Error(w, http.StatusText(http.StatusForbidden), http.StatusForbidden)

	// Log the response status and time taken.
	elapsed := time.Since(start)
	dlog.Printf("[DMSGHTTP] %s %s | %d | %v | %s | %s %s\n", start.Format("2006/01/02 - 15:04:05"), r.RemoteAddr, http.StatusForbidden, elapsed, r.Method, r.Proto, r.URL)
}

// Execute executes root CLI command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// WHY WON'T THIS PRINT??
		dlog.WithError(err).Debug("An error occurred\n")
		log.Fatal("Failed to execute command: ", err)
	}
}
