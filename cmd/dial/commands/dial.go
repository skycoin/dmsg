// Package commands cmd/dial/commands/dial.go
package commands

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

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
	sk          cipher.SecKey
	dmsgServers []string
)

func init() {
	flags.InitFlags(RootCmd)
	RootCmd.Flags().StringSliceVarP(&dmsgServers, "srv", "d", []string{}, "dmsg server public keys")
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
	Run: func(_ *cobra.Command, _ []string) {
		dlog := logging.MustGetLogger("dmsgdial")
		lvl, err := logging.LevelFromString("debug")
		if err == nil {
			logging.SetLevel(lvl)
		}

		pk, err := sk.PubKey()
		if err != nil {
			pk, sk = cipher.GenerateKeyPair()
		}

		ctx, cancel := cmdutil.SignalContext(context.Background(), dlog)
		defer cancel()

		httpClient := &http.Client{}

		var closeDmsg func()

		if flags.UseDC {
			_, closeDmsg, err = cli.StartDmsgDirect(ctx, dlog, pk, sk, httpClient, "", flags.DmsgSessions, pk.String())
		} else {
			if flags.UseHTTP {
				_, closeDmsg, err = cli.StartDmsg(ctx, dlog, pk, sk, httpClient, flags.DmsgDiscURL, flags.DmsgSessions)
			} else {
				var dmsgDC *dmsg.Client
				var closeDmsgDC func()
				dmsgDC, closeDmsgDC, err = cli.StartDmsgDirect(ctx, dlog, pk, sk, httpClient, "", flags.DmsgSessions, dmsg.ExtractPKFromDmsgAddr(flags.DmsgDiscAddr))
				if err != nil {
					dlog.WithError(err).Error("Error connecting to dmsg network")
					return
				}
				defer closeDmsgDC()
				dmsgHTTP := &http.Client{Transport: dmsghttp.MakeHTTPTransport(ctx, dmsgDC)}
				_, closeDmsg, err = cli.StartDmsg(ctx, dlog, pk, sk, dmsgHTTP, flags.DmsgDiscAddr, flags.DmsgSessions)
			}
		}
		if err != nil {
			dlog.WithError(err).Error("Error connecting to dmsg network")
		}
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
