// +build !windows

package cmdutil

import (
	"os"

	"golang.org/x/sys/unix"
)

func ignoreSignals() []os.Signal {
	return []os.Signal{unix.SIGINT, unix.SIGTERM, unix.SIGQUIT}
}
