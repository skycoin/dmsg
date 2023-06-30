// package main cmd/dmsgweb/dmsgweb.go
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
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
	httpC        http.Client
	dmsgDisc     string
	dmsgSessions int
	sk           cipher.SecKey
	dmsggetLog   *logging.Logger
	dmsggetAgent string
	logLvl       string
	webPort      string
)

func init() {
	rootCmd.Flags().StringVarP(&webPort, "port", "p", ":8080", "port to serve the web application")
	rootCmd.Flags().StringVarP(&dmsgDisc, "dmsg-disc", "d", "", "dmsg discovery url default:\n"+skyenv.DmsgDiscAddr)
	rootCmd.Flags().IntVarP(&dmsgSessions, "sess", "e", 1, "number of dmsg servers to connect to")
	rootCmd.Flags().StringVarP(&logLvl, "loglvl", "l", "", "[ debug | warn | error | fatal | panic | trace | info ]\033[0m")
	rootCmd.Flags().StringVarP(&dmsggetAgent, "agent", "a", "dmsgget/"+buildinfo.Version(), "identify as `AGENT`")
	if os.Getenv("DMSGGET_SK") != "" {
		sk.Set(os.Getenv("DMSGGET_SK")) //nolint
	}
	rootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r")
	var helpflag bool
	rootCmd.SetUsageTemplate(help)
	rootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for "+rootCmd.Use)
	rootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	rootCmd.PersistentFlags().MarkHidden("help") //nolint
}

var rootCmd = &cobra.Command{
	Use:   "dmsgweb",
	Short: "access websites over dmsg",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐┬ ┬┌─┐┌┐
	 │││││└─┐│ ┬│││├┤ ├┴┐
	─┴┘┴ ┴└─┘└─┘└┴┘└─┘└─┘`,
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
	Run: func(cmd *cobra.Command, _ []string) {

		if dmsggetLog == nil {
			dmsggetLog = logging.MustGetLogger("dmsgget")
		}
		if logLvl != "" {
			if lvl, err := logging.LevelFromString(logLvl); err == nil {
				logging.SetLevel(lvl)
			}
		}
		ctx, cancel := cmdutil.SignalContext(context.Background(), dmsggetLog)
		defer cancel()

		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}

		dmsgC, closeDmsg, err := startDmsg(ctx, pk, sk)
		if err != nil {
			dmsggetLog.WithError(err).Fatal("failed to start dmsg")
		}
		defer closeDmsg()

		httpC = http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgC)}

		r := gin.New()

		r.Use(gin.Recovery())

		r.Use(loggingMiddleware())

		r.Any("/*path", func(c *gin.Context) {
			fmt.Println(c.Request.Host)
			urlStr := "dmsg://" + strings.TrimRight(strings.TrimRight(c.Request.Host, "dmsg"), ".") + ":80" + c.Param("path")
			maxSize := int64(0)

			fmt.Println(urlStr)
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

			if maxSize > 0 {
				if resp.ContentLength > maxSize*1024 {
					c.String(http.StatusRequestEntityTooLarge, "Requested file size exceeds the allowed limit")
					return
				}
			}
			c.Status(http.StatusOK)
			io.Copy(c.Writer, resp.Body) //nolint
		})
		r.Run(webPort) //nolint
	},
}

func startDmsg(ctx context.Context, pk cipher.PubKey, sk cipher.SecKey) (dmsgC *dmsg.Client, stop func(), err error) {
	dmsgC = dmsg.NewClient(pk, sk, disc.NewHTTP(dmsgDisc, &http.Client{}, dmsggetLog), &dmsg.Config{MinSessions: dmsgSessions})
	go dmsgC.Serve(context.Background())

	stop = func() {
		err := dmsgC.Close()
		dmsggetLog.WithError(err).Debug("Disconnected from dmsg network.")
		fmt.Printf("\n")
	}
	dmsggetLog.WithField("public_key", pk.String()).WithField("dmsg_disc", dmsgDisc).
		Debug("Connecting to dmsg network...")

	select {
	case <-ctx.Done():
		stop()
		return nil, nil, ctx.Err()

	case <-dmsgC.Ready():
		dmsggetLog.Debug("Dmsg network ready.")
		return dmsgC, stop, nil
	}
}

func loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		if latency > time.Minute {
			latency = latency.Truncate(time.Second)
		}
		statusCode := c.Writer.Status()
		method := c.Request.Method
		path := c.Request.URL.Path
		// Get the background color based on the status code
		statusCodeBackgroundColor := getBackgroundColor(statusCode)
		// Get the method color
		methodColor := getMethodColor(method)
		// Print the logging in a custom format which includes the publickeyfrom c.Request.RemoteAddr ex.:
		// [DMSGHTTP] 2023/05/18 - 19:43:15 | 200 |    10.80885ms |                 | 02b5ee5333aa6b7f5fc623b7d5f35f505cb7f974e98a70751cf41962f84c8c4637:49153 | GET      /node-info.json
		fmt.Printf("[DMSGHTTP] %s |%s %3d %s| %13v | %15s | %72s |%s %-7s %s %s\n",
			time.Now().Format("2006/01/02 - 15:04:05"),
			statusCodeBackgroundColor,
			statusCode,
			resetColor(),
			latency,
			c.ClientIP(),
			c.Request.RemoteAddr,
			methodColor,
			method,
			resetColor(),
			path,
		)
	}
}
func getBackgroundColor(statusCode int) string {
	switch {
	case statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices:
		return green
	case statusCode >= http.StatusMultipleChoices && statusCode < http.StatusBadRequest:
		return white
	case statusCode >= http.StatusBadRequest && statusCode < http.StatusInternalServerError:
		return yellow
	default:
		return red
	}
}

func getMethodColor(method string) string {
	switch method {
	case http.MethodGet:
		return blue
	case http.MethodPost:
		return cyan
	case http.MethodPut:
		return yellow
	case http.MethodDelete:
		return red
	case http.MethodPatch:
		return green
	case http.MethodHead:
		return magenta
	case http.MethodOptions:
		return white
	default:
		return reset
	}
}

func resetColor() string {
	return reset
}

const (
	green   = "\033[97;42m"
	white   = "\033[90;47m"
	yellow  = "\033[90;43m"
	red     = "\033[97;41m"
	blue    = "\033[97;44m"
	magenta = "\033[97;45m"
	cyan    = "\033[97;46m"
	reset   = "\033[0m"
)

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
