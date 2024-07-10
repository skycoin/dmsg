// Package main cmd/dmsg/dmsg.go
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"

	dmsgdisc "github.com/skycoin/dmsg/cmd/dmsg-discovery/commands"
	dmsgserver "github.com/skycoin/dmsg/cmd/dmsg-server/commands"
	dmsgsocks "github.com/skycoin/dmsg/cmd/dmsg-socks5/commands"
	dmsgcurl "github.com/skycoin/dmsg/cmd/dmsgcurl/commands"
	dmsghttp "github.com/skycoin/dmsg/cmd/dmsghttp/commands"
	dmsgip "github.com/skycoin/dmsg/cmd/dmsgip/commands"
	dmsgptycli "github.com/skycoin/dmsg/cmd/dmsgpty-cli/commands"
	dmsgptyhost "github.com/skycoin/dmsg/cmd/dmsgpty-host/commands"
	dmsgptyui "github.com/skycoin/dmsg/cmd/dmsgpty-ui/commands"
	dmsgweb "github.com/skycoin/dmsg/cmd/dmsgweb/commands"
)

func init() {
	dmsgptyCmd.AddCommand(
		dmsgptycli.RootCmd,
		dmsgptyhost.RootCmd,
		dmsgptyui.RootCmd,
	)
	RootCmd.AddCommand(
		dmsgptyCmd,
		dmsgdisc.RootCmd,
		dmsgserver.RootCmd,
		dmsghttp.RootCmd,
		dmsgcurl.RootCmd,
		dmsgweb.RootCmd,
		dmsgsocks.RootCmd,
		dmsgip.RootCmd,
	)
	dmsgdisc.RootCmd.Use = "disc"
	dmsgserver.RootCmd.Use = "server"
	dmsghttp.RootCmd.Use = "http"
	dmsgcurl.RootCmd.Use = "curl"
	dmsgweb.RootCmd.Use = "web"
	dmsgsocks.RootCmd.Use = "socks"
	dmsgptycli.RootCmd.Use = "cli"
	dmsgptyhost.RootCmd.Use = "host"
	dmsgptyui.RootCmd.Use = "ui"
	dmsgip.RootCmd.Use = "ip"

	var helpflag bool
	RootCmd.SetUsageTemplate(help)
	RootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for dmsg")
	RootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	RootCmd.PersistentFlags().MarkHidden("help") //nolint
	RootCmd.CompletionOptions.DisableDefaultCmd = true

}

// RootCmd contains all binaries which may be separately compiled as subcommands
var RootCmd = &cobra.Command{
	Use: func() string {
		return strings.Split(filepath.Base(strings.ReplaceAll(strings.ReplaceAll(fmt.Sprintf("%v", os.Args), "[", ""), "]", "")), " ")[0]
	}(),
	Short: "DMSG services & utilities",
	Long: `
	┌┬┐┌┬┐┌─┐┌─┐
	 │││││└─┐│ ┬
	─┴┘┴ ┴└─┘└─┘
DMSG services & utilities`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
}

var dmsgptyCmd = &cobra.Command{
	Use:   "pty",
	Short: "DMSG pseudoterminal (pty)",
	Long: `
	┌─┐┌┬┐┬ ┬
	├─┘ │ └┬┘
	┴   ┴  ┴
DMSG pseudoterminal (pty)`,
	SilenceErrors:         true,
	SilenceUsage:          true,
	DisableSuggestions:    true,
	DisableFlagsInUseLine: true,
}

func main() {
	cc.Init(&cc.Config{
		RootCmd:         RootCmd,
		Headings:        cc.HiBlue + cc.Bold,
		Commands:        cc.HiBlue + cc.Bold,
		CmdShortDescr:   cc.HiBlue,
		Example:         cc.HiBlue + cc.Italic,
		ExecName:        cc.HiBlue + cc.Bold,
		Flags:           cc.HiBlue + cc.Bold,
		FlagsDescr:      cc.HiBlue,
		NoExtraNewlines: true,
		NoBottomNewline: true,
	})

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
	}
}

const help = "{{if gt (len .Aliases) 0}}" +
	"{{.NameAndAliases}}{{end}}{{if .HasAvailableSubCommands}}" +
	"Available Commands:{{range .Commands}}{{if (or .IsAvailableCommand)}}\r\n  " +
	"{{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}\r\n\r\n" +
	"Flags:\r\n" +
	"{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}{{if .HasAvailableInheritedFlags}}\r\n\r\n" +
	"Global Flags:\r\n" +
	"{{.InheritedFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}\r\n\r\n"
