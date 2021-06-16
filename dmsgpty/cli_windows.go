//+build windows

package dmsgpty

import (
	"context"

	"github.com/ActiveState/termtest/conpty"
)

// ptyResizeLoop informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(_ context.Context, ptyC *PtyClient) error {
	// TODO: resize windows pty
	return nil
}

func (cli *CLI) prepareStdin() (restore func(), err error) {
	return conpty.InitTerminal(true)
}
