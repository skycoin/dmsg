//+build windows

package dmsgpty

import (
	"context"
	"os"

	"golang.org/x/sys/windows"
)

// ptyResizeLoop informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(_ context.Context, ptyC *PtyClient) error {
	// TODO: resize windows pty
	return nil
}

// getPtySize obtains the size of the local terminal.
func getPtySize(_ *os.File) (*windows.Coord, error) {
	return getSize()
}
