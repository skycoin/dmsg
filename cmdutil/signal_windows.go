// +build windows

package cmdutil

import (
	"os"

	"golang.org/x/sys/windows"
)

func ignoreSignals() []os.Signal {
	return []os.Signal{windows.SIGINT, windows.SIGTERM, windows.SIGQUIT}
}
