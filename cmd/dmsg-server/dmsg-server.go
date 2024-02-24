// package main cmd/dmsg-server/dmsg-server.go
package main

import (
	cc "github.com/ivanpirog/coloredcobra"
	"github.com/spf13/cobra"

	"github.com/skycoin/dmsg/cmd/dmsg-server/commands"
)

func init() {
	commands.RootCmd.SetUsageTemplate(help)
	var helpflag bool
	commands.RootCmd.PersistentFlags().BoolVarP(&helpflag, "help", "h", false, "help for dmsg-server")
	commands.RootCmd.SetHelpCommand(&cobra.Command{Hidden: true})
	commands.RootCmd.PersistentFlags().MarkHidden("help") //nolint
}

func main() {
	cc.Init(&cc.Config{
		RootCmd:       commands.RootCmd,
		Headings:      cc.HiBlue + cc.Bold,
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
	commands.Execute()
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
