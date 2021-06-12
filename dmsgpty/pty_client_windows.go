//+build windows

package dmsgpty

import (
	"os"

	"github.com/containerd/console"
)

// Start starts the pty.
func (sc *PtyClient) Start(name string, arg ...string) error {
	c, err := console.ConsoleFromFile(os.Stdin)
	if err != nil {
		sc.log.WithError(err).Warn("failed to obtain terminal size")
		c = nil
	}
	return sc.StartWithSize(name, arg, c)
}

// StartWithSize starts the pty with a specified size.
func (sc *PtyClient) StartWithSize(name string, arg []string, c console.Console) error {
	return sc.call("Start", &CommandReq{Name: name, Arg: arg, Size: c}, &empty)
}

// SetPtySize sets the pty size.
func (sc *PtyClient) SetPtySize(size console.WinSize) error {
	return sc.call("SetPtySize", size, &empty)
}
