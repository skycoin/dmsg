// package main cmd/dmsg-server/dmsg-server.go
package main

import (
	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/flags"

	"github.com/skycoin/dmsg/cmd/dmsg-server/commands"
)

func init() {
	flags.InitFlags(commands.RootCmd, true)
}

func main() {
	commands.Execute()
}
