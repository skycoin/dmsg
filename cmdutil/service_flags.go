package cmdutil

import (
	"github.com/spf13/cobra"
)

type ServiceFlags struct {
	MetricsAddr string
	SyslogNet   string
	SyslogAddr  string
	SyslogLvl   string
	Tag         string
	Stdin       bool
}

func InitServiceFlags(rootCmd *cobra.Command, flags *ServiceFlags) {
	rootCmd.Flags().StringVar(&flags.MetricsAddr,
		"metrics", "", "address to serve metrics API from")
	rootCmd.Flags().StringVar(&flags.SyslogNet,
		"syslog-net", "tcp", "network in which to dial to syslog server")
	rootCmd.Flags().StringVar(&flags.SyslogAddr,
		"syslog-addr", "", "address in which to dial to syslog server")
	rootCmd.Flags().StringVar(&flags.SyslogLvl,
		"syslog-lvl", "", "")
}
