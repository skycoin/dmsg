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

	"github.com/sirupsen/logrus"
	"github.com/skycoin/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	"github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

var (
	dmsgDisc       []string
	dmsgSessions   int
	dmsgcurlData   string
	sk             cipher.SecKey
	dmsgcurlLog    *logging.Logger
	dmsgcurlAgent  string
	logLvl         string
	dmsgcurlTries  int
	dmsgcurlWait   int
	dmsgcurlOutput string
	replace        bool
)

func init() {
	RootCmd.Flags().StringSliceVarP(&dmsgDisc, "dmsg-disc", "c", []string{skyenv.DmsgDiscAddr}, "dmsg discovery url(s)")
	RootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to")
	RootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "fatal", "[ debug | warn | error | fatal | panic | trace | info ]")
	RootCmd.Flags().StringVarP(&dmsgcurlData, "data", "d", "", "dmsghttp POST data")
	RootCmd.Flags().StringVarP(&dmsgcurlOutput, "out", "o", "", "output filepath")
	RootCmd.Flags().BoolVarP(&replace, "replace", "r", false, "replace existing file with new downloaded")
	RootCmd.Flags().IntVarP(&dmsgcurlTries, "try", "t", 1, "download attempts (0 unlimits)")
	RootCmd.Flags().IntVarP(&dmsgcurlWait, "wait", "w", 0, "time to wait between fetches")
	RootCmd.Flags().StringVarP(&dmsgcurlAgent, "agent", "a", "dmsgcurl/"+buildinfo.Version(), "identify as `AGENT`")
	if os.Getenv("DMSGCURL_SK") != "" {
		sk.Set(os.Getenv("DMSGCURL_SK")) //nolint
	}
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified")
}

// RootCmd contains the root cli command
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Short:                 "DMSG curl utility",
	Long:                  `DMSG curl utility`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	RunE: func(_ *cobra.Command, args []string) error {
		if len(dmsgDisc) == 0 || dmsgDisc[0] == "" {
			dmsgDisc = []string{skyenv.DmsgDiscAddr}
		}
		if dmsgcurlLog == nil {
			dmsgcurlLog = logging.MustGetLogger("dmsgcurl")
		}
		if logLvl != "" {
			if lvl, err := logging.LevelFromString(logLvl); err == nil {
				logging.SetLevel(lvl)
			}
		}
		ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgcurlLog)
		defer cancel()
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
			dmsgcurlLog.WithError(err).Fatal("failed to parse provided URL")
		}
		if dmsgcurlData != "" {
			return handlePostRequest(ctx, pk, parsedURL)
		}
		return handleDownload(ctx, pk, parsedURL)
	},
}

func handlePostRequest(ctx context.Context, pk cipher.PubKey, parsedURL *url.URL) error {
	for _, disco := range dmsgDisc {
		dmsgC, closeDmsg, err := startDmsg(ctx, pk, sk, disco)
		if err != nil {
			dmsgcurlLog.WithError(err).Warnf("Failed to start dmsg with discovery %s", disco)
			continue
		}
		defer closeDmsg()

		httpC := http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}
		req, err := http.NewRequest(http.MethodPost, parsedURL.String(), strings.NewReader(dmsgcurlData))
		if err != nil {
			dmsgcurlLog.WithError(err).Fatal("Failed to formulate HTTP request.")
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpC.Do(req)
		if err != nil {
			dmsgcurlLog.WithError(err).Warnf("Failed to execute HTTP request with discovery %s", disco)
			continue
		}
		defer closeResponseBody(resp)

		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			dmsgcurlLog.WithError(err).Fatal("Failed to read response body.")
		}
		fmt.Println(string(respBody))
		return nil
	}
	return errors.New("all dmsg discovery addresses failed")
}

func handleDownload(ctx context.Context, pk cipher.PubKey, parsedURL *url.URL) error {
	file, err := prepareOutputFile()
	if err != nil {
		return fmt.Errorf("failed to prepare output file: %w", err)
	}
	defer closeAndCleanFile(file, err)

	for _, disco := range dmsgDisc {
		dmsgC, closeDmsg, err := startDmsg(ctx, pk, sk, disco)
		if err != nil {
			dmsgcurlLog.WithError(err).Warnf("Failed to start dmsg with discovery %s", disco)
			continue
		}
		defer closeDmsg()

		httpC := http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}

		for i := 0; i < dmsgcurlTries; i++ {
			if dmsgcurlOutput != "" {
				dmsgcurlLog.Debugf("Download attempt %d/%d ...", i, dmsgcurlTries)
				if _, err := file.Seek(0, 0); err != nil {
					return fmt.Errorf("failed to reset file: %w", err)
				}
			}
			if err := download(ctx, dmsgcurlLog, &httpC, file, parsedURL.String(), 0); err != nil {
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
	}

	return errors.New("all download attempts failed with all dmsg discovery addresses")
}

func prepareOutputFile() (*os.File, error) {
	if dmsgcurlOutput == "" {
		return os.Stdout, nil
	}
	return parseOutputFile(dmsgcurlOutput, replace)
}

func closeAndCleanFile(file *os.File, err error) {
	if fErr := file.Close(); fErr != nil {
		dmsgcurlLog.WithError(fErr).Warn("Failed to close output file.")
	}
	if err != nil && file != os.Stdout {
		if rErr := os.RemoveAll(file.Name()); rErr != nil {
			dmsgcurlLog.WithError(rErr).Warn("Failed to remove output file.")
		}
	}
}

func closeResponseBody(resp *http.Response) {
	if err := resp.Body.Close(); err != nil {
		dmsgcurlLog.WithError(err).Fatal("Failed to close response body")
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
		return os.OpenFile(filepath.Clean(output), os.O_RDWR|os.O_CREATE|os.O_TRUNC, os.ModePerm)
	}
	return nil, os.ErrExist
}

func startDmsg(ctx context.Context, pk cipher.PubKey, sk cipher.SecKey, disco string) (dmsgC *dmsg.Client, stop func(), err error) {
	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(disco, &http.Client{}, dmsgcurlLog), &dmsg.Config{MinSessions: dmsgSessions})
	go dmsgC.Serve(context.Background())

	stop = func() {
		err := dmsgC.Close()
		dmsgcurlLog.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
	dmsgcurlLog.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc).
		Debug("Connecting to dmsg network...")

	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsgcurlLog.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}

func download(ctx context.Context, log logrus.FieldLogger, httpC *http.Client, w io.Writer, urlStr string, maxSize int64) error {
	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		log.WithError(err).Fatal("Failed to formulate HTTP request.")
	}
	resp, err := httpC.Do(req)
	if err != nil {
		return fmt.Errorf("failed to connect to HTTP server: %w", err)
	}
	if maxSize > 0 && resp.ContentLength > maxSize*1024 {
		return fmt.Errorf("requested file size is more than allowed size: %d KB > %d KB", (resp.ContentLength / 1024), maxSize)
	}
	n, err := cancellableCopy(ctx, w, resp.Body, resp.ContentLength)
	if err != nil {
		return fmt.Errorf("download failed at %d/%dB: %w", n, resp.ContentLength, err)
	}
	defer closeResponseBody(resp)

	return nil
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
		log.Fatal("Failed to execute command: ", err)
	}
}
