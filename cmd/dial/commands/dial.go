// Package commands cmd/dial/commands/dial.go
package commands

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/chen3feng/safecast"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/buildinfo"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/calvin"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cmdutil"
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/logging"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/internal/cli"
	"github.com/skycoin/dmsg/internal/flags"
	"github.com/skycoin/dmsg/pkg/dmsg"
	"github.com/skycoin/dmsg/pkg/dmsghttp"
)

var (
	sk       cipher.SecKey
	dpk      cipher.PubKey
	waitTime int
	dport    uint
)

func init() {
	flags.InitFlags(RootCmd)
	RootCmd.Flags().IntVarP(&waitTime, "wait", "w", 0, "wait time in seconds before disconnecting\n\r\033[0m")
	RootCmd.Flags().VarP(&sk, "sk", "s", "a random key is generated if unspecified\n\r\033[0m")
}

// RootCmd contains the root dmsgcurl command
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Short: "DMSG Dial utility",
	Long: calvin.AsciiFont("dmsgdial") + `
	DMSG Dial network test utility

	Test connection to dmsg server`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
	Version:               buildinfo.Version(),
	Run: func(_ *cobra.Command, args []string) {
		dlog := logging.MustGetLogger("dmsgdial")
		lvl, err := logging.LevelFromString("debug")
		if err == nil {
			logging.SetLevel(lvl)
		}

		pk, err := sk.PubKey()
		if err != nil {
			_, sk = cipher.GenerateKeyPair()
			pk, err = sk.PubKey()
			if err != nil {
				dlog.WithError(err).Fatal("Failed to derive public key from secret key")
			}
		}

		if len(args) > 0 && strings.Contains(args[0], ":") {
			parts := strings.Split(args[0], ":")
			if len(parts) < 1 || parts[0] == "" {
				dlog.Fatal("Invalid dmsg address format. Expected <public_key>[:<port>]")
			}

			// Parse the public key
			if err := dpk.Set(parts[0]); err != nil {
				dlog.WithError(err).Fatal("Failed to parse public key from dmsg address")
			}
			dlog.Info("Parsed dmsg client public key to dial: ", dpk.String())
			// Parse the port or use the default (80)
			dport = uint(80) // Default port
			if len(parts) > 1 && parts[1] != "" {
				parsedPort, err := strconv.ParseUint(parts[1], 10, 16) // Ports are 16-bit unsigned integers
				if err != nil {
					dlog.WithError(err).Fatal("Failed to parse dmsg port")
				}
				dport = uint(parsedPort)
			}
			dlog.Info("Parsed dmsg client port to dial: ", dport)
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), dlog)
		defer cancel()

		httpClient := &http.Client{}
		dmsgC := &dmsg.Client{}
		var closeDmsg func()

		if flags.UseDC {
			dmsgC, closeDmsg, err = cli.StartDmsgDirect(ctx, dlog, pk, sk, httpClient, "", flags.DmsgSessions, pk.String())
		} else {
			if flags.UseHTTP {
				resp, err := httpClient.Get(flags.DmsgDiscURL + "/health")
				if err != nil {
					dlog.WithError(err).Fatal("Error connecting to dmsg-discovery with http client")
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					dlog.WithError(err).Error("Failed to read response body from discovery")
				} else {
					dlog.Infof("Received response from dmsg-discovery server %s/health:\n%s", flags.DmsgDiscURL, string(body))
				}

				dmsgC, closeDmsg, err = cli.StartDmsg(ctx, dlog, pk, sk, httpClient, flags.DmsgDiscURL, flags.DmsgSessions)
			} else {
				//Default dmsghttp mode
				var dmsgDC *dmsg.Client
				var closeDmsgDC func()
				dmsgDC, closeDmsgDC, err = cli.StartDmsgDirect(ctx, dlog, pk, sk, httpClient, flags.DmsgDiscAddr, flags.DmsgSessions, dmsg.ExtractPKFromDmsgAddr(flags.DmsgDiscAddr))
				if err != nil {
					dlog.WithError(err).Error("Error connecting to dmsg network")
					return
				}
				defer closeDmsgDC()
				dmsgHTTP := &http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgDC)}

				resp, err := dmsgHTTP.Get(flags.DmsgDiscAddr + "/health")
				if err != nil {
					dlog.WithError(err).Fatal("Error connecting to dmsg-discovery with dmsg direct client via current dmsg server.")
				}
				defer resp.Body.Close()

				body, err := io.ReadAll(resp.Body)
				if err != nil {
					dlog.WithError(err).Error("Failed to read response body from dmsg-discovery")
				} else {
					dlog.Infof("Received response from dmsg-discovery server %s/health:\n%s", flags.DmsgDiscURL, string(body))
				}
				dmsgC, closeDmsg, err = cli.StartDmsg(ctx, dlog, pk, sk, dmsgHTTP, flags.DmsgDiscAddr, flags.DmsgSessions)
			}
		}
		if err != nil {
			dlog.WithError(err).Error("Error connecting to dmsg network")
			return
		}
		if len(args) > 0 {
			dlog.Debug(fmt.Sprintf("Dialing %v:%v", dpk.String(), dport))
			dp, ok := safecast.To[uint16](dport)
			if !ok {
				dlog.Fatal("uint16 overflow when converting dmsg port")
			}
			dmsgConn, err := dmsgC.DialStream(context.Background(), dmsg.Addr{PK: dpk, Port: dp}) //nolint
			if err != nil {
				dlog.WithError(err).Warn(fmt.Sprintf("Failed to dial dmsg address %v port %v", dpk.String(), dp))
			}
			err = dmsgConn.Close() //nolint
			if err != nil {
				dlog.WithError(err).Error("Error closing dmsg client connection")
			}
		}

		time.Sleep(time.Duration(waitTime) * time.Second)
		dlog.Info("Disconnecting from dmsg network")
		closeDmsg()
	},
}

// Execute executes root CLI command.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		log.Fatal("Failed to execute command: ", err)
	}
}
