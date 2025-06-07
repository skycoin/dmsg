// Package main cmd/conf/conf.go
package main

import (
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/flags"

	"github.com/skycoin/dmsg/cmd/conf/commands"
)

func init() {
	flags.InitFlags(commands.RootCmd, true)
}

func main() {
	commands.Execute()
}
