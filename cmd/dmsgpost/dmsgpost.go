// package main cmd/dmsgpost/dmsgpost.go
package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"

	"strings"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/skycoin/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire-utilities/pkg/logging"
	"github.com/skycoin/skywire-utilities/pkg/skyenv"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/pkg/disc"
	dmsg "github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

var (
	dmsgDisc      string
	dmsgSessions  int
	dmsgpostTries  int
	dmsgpostWait   int
	dmsgpostData string
//	dmsgpostHeader string
	sk            cipher.SecKey
	dmsgpostLog    *logging.Logger
	dmsgpostAgent  string
	logLvl        string
)

func init() {
	rootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "c", "", "dmsg discovery url default:\n"+skyenv.DmsgDiscAddr)
	rootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to")
	rootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m")
	rootCmd.Flags().StringVarP(&dmsgpostData, "data", "d", "", "dmsghttp POST data")
//	rootCmd.Flags().StringVarP(&dmsgpostHeader, "header", "H", "", "Pass custom header(s) to server")
	rootCmd.Flags().StringVarP(&dmsgpostAgent, "agent", "a", "dmsgpost/"+buildinfo.Version(), "identify as `AGENT`")
	if os.Getenv("dmsgpost_SK") != "" {
		sk.Set(os.Getenv("dmsgpost_SK")) //nolint
	}
	rootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")
	var helpflag bool
	rootCmd.SetUsageTemplate(help)
	rootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for "+rootCmd.Use)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().MarkHidden("help") //nolint
}

var rootCmd = &cobra.Command{
	Use:   "dmsgpost",
	Short: "dmsgpost",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐┌─┐┌─┐┌─┐┌┬┐
 	 │││││└─┐│ ┬├─┘│ │└─┐ │
	─┴┘┴ ┴└─┘└─┘┴  └─┘└─┘ ┴ `,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	PreRun: func(cmd *cobra.Command, args []string) {
		if dmsgDisc == "" {
			dmsgDisc = skyenv.DmsgDiscAddr
		}
	},
	Run: func(cmd *cobra.Command, args []string) {
		if dmsgpostLog == nil {
			dmsgpostLog = logging.MustGetLogger("dmsgpost")
		}
		if logLvl != "" {
			if lvl, err := logging.LevelFromString(logLvl); err == nil {
				logging.SetLevel(lvl)
			}
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), dmsgpostLog)
		defer cancel()

		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}

		u, err := parseURL(args)
		if err != nil {
			dmsgpostLog.WithError(err).Fatal("failed to parse provided URL")
		}

		dmsgC, closeDmsg, err := startDmsg(ctx, pk, sk)
		if err != nil {
			dmsgpostLog.WithError(err).Fatal("failed to start dmsg")
		}
		defer closeDmsg()

		httpC := http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}

		req, err := http.NewRequest(http.MethodPost, u.URL.String(), strings.NewReader(dmsgpostData))
		if err != nil {
			dmsgpostLog.WithError(err).Fatal("Failed to formulate HTTP request.")
		}
		req.Header.Set("Content-Type", "text/plain")

		resp, err := httpC.Do(req)
		if err != nil {
			dmsgpostLog.WithError(err).Fatal("Failed to execute HTTP request.")
		}

		defer resp.Body.Close()
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			dmsgpostLog.WithError(err).Fatal("Failed to read respose body.")
		}
		fmt.Println(string(respBody))
	},
}

// URL represents a dmsg http URL.
type URL struct {
	dmsg.Addr
	url.URL
}

// Fill fills the internal fields from an URL string.
func (du *URL) fill(str string) error {
	u, err := url.Parse(str)
	if err != nil {
		return err
	}

	if u.Scheme == "" {
		return errors.New("URL is missing a scheme")
	}

	if u.Host == "" {
		return errors.New("URL is missing a host")
	}

	du.URL = *u
	return du.Addr.Set(u.Host)
}

func parseURL(args []string) (*URL, error) {
	if len(args) == 0 {
		return nil, errors.New("no URL(s) provided")
	}

	if len(args) > 1 {
		return nil, errors.New("multiple URLs is not yet supported")
	}

	var out URL
	if err := out.fill(args[0]); err != nil {
		return nil, fmt.Errorf("provided URL is invalid: %w", err)
	}

	return &out, nil
}


func startDmsg(ctx context.Context, pk cipher.PubKey, sk cipher.SecKey) (dmsgC *dmsg.Client, stop func(), err error) {
	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, dmsgpostLog), &dmsg.Config{MinSessions: dmsgSessions})
	go dmsgC.Serve(context.Background())

	stop = func() {
		err := dmsgC.Close()
		dmsgpostLog.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
		dmsgpostLog.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc).
			Debug("Connecting to dmsg network...")

	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsgpostLog.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}

// Execute executes root CLI command.
func Execute() {
	cc.Init(&cc.Config{
		RootCmd:       rootCmd,
		Headings:      cc.HiBlue + cc.Bold, //+ cc.Underline,
		Commands:      cc.HiBlue + cc.Bold,
		CmdShortDescr: cc.HiBlue,
		Example:       cc.HiBlue + cc.Italic,
		ExecName:      cc.HiBlue + cc.Bold,
		Flags:         cc.HiBlue + cc.Bold,
		//FlagsDataType: cc.HiBlue,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})
	if err := rootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
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

func main() {
	Execute()
}
