// Package main cmd/dmsgpty-ui/dmsgpty-ui.go
package main

import (
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/flags"

	"github.com/skycoin/dmsg/cmd/dmsgpty-ui/commands"
)

func init() {
	flags.InitFlags(commands.RootCmd, true)
}

func main() {
	commands.Execute()
}
