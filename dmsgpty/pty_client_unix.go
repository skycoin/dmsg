//+build !windows

package dmsgpty

import (
	"os"

	"github.com/creack/pty"
)

// Start starts the pty.
func (sc *PtyClient) Start(name string, arg ...string) error {
	size, err := pty.GetsizeFull(os.Stdin)
	if err != nil {
		sc.log.WithError(err).Warn("failed to obtain terminal size")
		size = nil
	}
	return sc.StartWithSize(name, arg, newWinSize(size))
}
