// package main cmd/dmsg-discovery/dmsg-discovery.go
package main

import (
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/flags"

	"github.com/skycoin/dmsg/cmd/dmsghttp/commands"
)

func init() {
	flags.InitFlags(commands.RootCmd, true)
}

func main() {
	commands.Execute()
}
