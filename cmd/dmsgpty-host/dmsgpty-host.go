// Package main cmd/dmsgpty-host/dmsgpty-host.go
package main

import (
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/flags"

	"github.com/skycoin/dmsg/cmd/dmsgpty-host/commands"
)

func init() {
	flags.InitFlags(commands.RootCmd, true)
}

func main() {
	commands.Execute()
}
