// Package commands cmd/dmsgcurl/commands
package commands

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"
	"golang.org/x/net/proxy"

	"github.com/skycoin/dmsg/internal/cli"
	"github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

var (
	ctxs           []context.Context
	cancels        []func()
	dmsgDiscs      []string
	dmsgSessions   int
	dmsgcurlData   string
	sk             cipher.SecKey
	dmsgcurlLog    = logging.MustGetLogger("dmsgcurl")
	dmsgcurlAgent  string
	logLvl         string
	dmsgcurlTries  int
	dmsgcurlWait   int
	dmsgcurlOutput string
	replace        bool
	proxyAddr      []string
	httpClients    []*http.Client
	dialer         = proxy.Direct
	dmsgHTTPPath   string
	useHTTP        bool
)

func init() {
	RootCmd.Flags().SortFlags = false
	RootCmd.Flags().BoolVarP(&useHTTP, "http", "z", false, "use regular http to connect to dmsg discovery")
	RootCmd.Flags().StringSliceVarP(&dmsgDiscs, "dmsg-disc", "c", []string{dmsg.DiscAddr(false)}, "dmsg discovery url(s)\033[0m\n\r")
	RootCmd.Flags().StringVarP(&dmsgHTTPPath, "dmsgconf", "D", "", "dmsghttp-config path")
	RootCmd.Flags().StringSliceVarP(&proxyAddr, "proxy", "p", proxyAddr, "connect to dmsg via proxy (i.e. '127.0.0.1:1080')")
	RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to\033[0m\n\r")
	RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "fatal", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m\n\r")
	RootCmd.Flags().StringVarP(&dmsgcurlData, "data", "d", "", "dmsghttp POST data")
	RootCmd.Flags().StringVarP(&dmsgcurlOutput, "out", "o", "", "output filepath")
	RootCmd.Flags().BoolVarP(&replace, "replace", "r", false, "replace existing file with new downloaded")
	RootCmd.Flags().IntVarP(&dmsgcurlTries, "try", "t", 1, "download attempts (0 unlimits)\033[0m\n\r")
	RootCmd.Flags().IntVarP(&dmsgcurlWait, "wait", "w", 0, "time to wait between requests")
	RootCmd.Flags().StringVarP(&dmsgcurlAgent, "agent", "a", "dmsgcurl/"+buildinfo.Version(), "identify as `AGENT`\033[0m\n\r")
	if os.Getenv("DMSGCURL_SK") != "" {
		sk.Set(os.Getenv("DMSGCURL_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\033[0m\n\r")
}

// RootCmd contains the root cli command
var RootCmd = &cobra.Command{
	Use:                   "curl",
	Short:                 "DMSG curl utility",
	Long:                  `DMSG curl utility`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	RunE: func(_ *cobra.Command, args []string) error {
		if logLvl != "" {
			if lvl, err := logging.LevelFromString(logLvl); err == nil {
				logging.SetLevel(lvl)
			}
		}
		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}
		if len(args) == 0 {
			return errors.New("no URL(s) provided")
		}
		if len(args) > 1 {
			return errors.New("multiple URLs is not yet supported")
		}
		parsedURL, err := url.Parse(args[0])
		if err != nil {
			dmsgcurlLog.WithError(err).Fatal("failed to parse provided URL\n")
		}
		if useHTTP {
			if len(dmsgDiscs) == 0 || dmsgDiscs[0] == "" {
				dmsgDiscs = []string{dmsg.DiscAddr(false)}
			}
			dmsgcurlLog.Debug("DMSG Discovery: ", dmsgDiscs)
			for i := range dmsgDiscs {
				ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgcurlLog)
				defer cancel()
				ctxs = append(ctxs, ctx)
				cancels = append(cancels, cancel)

				httpClient := &http.Client{}

				if i < len(proxyAddr) && proxyAddr[i] != "" {
					// Use SOCKS5 proxy dialer if specified
					dialer, err := proxy.SOCKS5("tcp", proxyAddr[i], nil, proxy.Direct)
					if err != nil {
						dmsgcurlLog.Fatalf("Error creating SOCKS5 dialer: %v", err)
					}
					transport := &http.Transport{
						Dial: dialer.Dial,
					}
					httpClient = &http.Client{
						Transport: transport,
					}
					ctxs[i] = context.WithValue(context.Background(), "socks5_proxy", proxyAddr[i]) //nolint
				}
				httpClients = append(httpClients, httpClient)

				err = handleRequest(ctxs[i], dmsgcurlLog, pk, sk, httpClients[i], dmsgDiscs[i], dmsgSessions, parsedURL, dmsgcurlData, !useHTTP)
				if err == nil {
					return nil
				}
				dmsgcurlLog.WithError(err).Debug("An error occurred\n")
			}
		} else { //Use direct dmsg client & embedded config
			ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgcurlLog)
			defer cancel()
			err = handleRequest(ctx, dmsgcurlLog, pk, sk, &http.Client{}, "", dmsgSessions, parsedURL, dmsgcurlData, !useHTTP)
			if err == nil {
				return nil
			}
			dmsgcurlLog.WithError(err).Debug("An error occurred\n")
		}
		return err
	},
}

func handleRequest(ctx context.Context, dmsgLogger *logging.Logger, pk cipher.PubKey, sk cipher.SecKey, httpClient *http.Client, dmsgDisc string, dmsgSessions int, parsedURL *url.URL, dmsgcurlData string, dmsgHTTP bool) error {
	file, err := prepareOutputFile()
	if err != nil {
		return fmt.Errorf("failed to prepare output file: %w", err)
	}
	defer closeAndCleanFile(file, err)
	var dmsgC *dmsg.Client
	var closeDmsg func()
	if !dmsgHTTP {
		dmsgC, closeDmsg, err = cli.StartDmsg(ctx, dmsgLogger, pk, sk, httpClient, dmsgDisc, dmsgSessions)
	} else {
		dmsgC, closeDmsg, err = cli.StartDmsgDirect(ctx, dmsgLogger, pk, sk, httpClient, dmsgDisc, dmsgSessions)
	}
	if err != nil {
		dmsgcurlLog.WithError(err).Fatal("Failed to start dmsg\n")
		return err
	}
	defer closeDmsg()

	if dmsgC == nil {
		dmsgcurlLog.Fatal("nil dmsg client pointer")
	}

	httpC := http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}
	firstTry := true
	for i := 0; i < dmsgcurlTries; i++ {
		if dmsgcurlOutput != "" {
			if !firstTry {
				dmsgcurlLog.Debugf("Download attempt %d/%d ...", i, dmsgcurlTries)
			}
			firstTry = false
			if _, err := file.Seek(0, 0); err != nil {
				return fmt.Errorf("failed to reset file: %w", err)
			}
		}
		var req *http.Request
		if dmsgcurlData != "" {
			req, err = http.NewRequest(http.MethodPost, parsedURL.String(), strings.NewReader(dmsgcurlData))
		} else {
			req, err = http.NewRequest(http.MethodGet, parsedURL.String(), nil)
		}
		if err != nil {
			return fmt.Errorf("Failed to formulate HTTP request: %w", err)
		}
		if dmsgcurlData != "" {
			req.Header.Set("Content-Type", "text/plain")
		}
		resp, err := httpC.Do(req)
		if err != nil {
			return fmt.Errorf("failed to preform HTTP request: %w", err)
		}
		//		if maxSize > 0 && resp.ContentLength > maxSize*1024 {
		//			return fmt.Errorf("requested file size is more than allowed size: %d KB > %d KB", (resp.ContentLength / 1024), maxSize)
		//		}
		n, err := cancellableCopy(ctx, file, resp.Body, resp.ContentLength)
		if err != nil {
			return fmt.Errorf("download failed at %d/%dB: %w", n, resp.ContentLength, err)
		}
		defer closeResponseBody(resp)
		if err != nil {
			dmsgcurlLog.WithError(err).Error()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(dmsgcurlWait) * time.Second):
				continue
			}
		}
		return nil
	}
	return err
}

func prepareOutputFile() (*os.File, error) {
	if dmsgcurlOutput == "" {
		return os.Stdout, nil
	}
	return parseOutputFile(dmsgcurlOutput, replace)
}

func closeAndCleanFile(file *os.File, err error) {
	if fErr := file.Close(); fErr != nil {
		dmsgcurlLog.WithError(fErr).Warn("Failed to close output file.\n")
	}
	if err != nil && file != os.Stdout {
		if rErr := os.RemoveAll(file.Name()); rErr != nil {
			dmsgcurlLog.WithError(rErr).Warn("Failed to remove output file.\n")
		}
	}
}

func closeResponseBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		dmsgcurlLog.WithError(err).Fatal("Failed to close response body\n")
	}
}

func parseOutputFile(output string, replace bool) (*os.File, error) {
	_, statErr := os.Stat(output)
	if statErr != nil {
		if os.IsNotExist(statErr) {
			if err := os.MkdirAll(filepath.Dir(output), fs.ModePerm); err != nil {
				return nil, err
			}
			f, err := os.Create(output) //nolint
			if err != nil {
				return nil, err
			}
			return f, nil
		}
		return nil, statErr
	}
	if replace {
		return os.OpenFile(filepath.Clean(output), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm) //nolint
	}
	return nil, os.ErrExist
}

type readerFunc func(p []byte) (n int, err error)

func (rf readerFunc) Read(p []byte) (n int, err error) { return rf(p) }

func cancellableCopy(ctx context.Context, w io.Writer, body io.ReadCloser, length int64) (int64, error) {
	n, err := io.Copy(io.MultiWriter(w, &progressWriter{Total: length}), readerFunc(func(p []byte) (int, error) {
		select {
		case <-ctx.Done():
			return 0, errors.New("Download Canceled")
		default:
			return body.Read(p)
		}
	}))
	return n, err
}

type progressWriter struct {
	Current int64
	Total   int64
}

func (pw *progressWriter) Write(p []byte) (int, error) {
	n := len(p)
	current := atomic.AddInt64(&pw.Current, int64(n))
	total := atomic.LoadInt64(&pw.Total)
	pc := fmt.Sprintf("%d%%", current*100/total)
	if dmsgcurlOutput != "" {
		fmt.Printf("Downloading: %d/%dB (%s)", current, total, pc)
		if current != total {
			fmt.Print("\r")
		} else {
			fmt.Print("\n")
		}
	}
	return n, nil
}

// Execute executes the RootCmd
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		// WHY WON'T THIS PRINT??
		dmsgcurlLog.WithError(err).Debug("An error occured\n")
		log.Fatal("Failed to execute command: ", err)
	}
}
