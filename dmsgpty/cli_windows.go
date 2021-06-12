//+build windows

package dmsgpty

import (
	"context"
	"fmt"
	"os"

	"github.com/containerd/console"
)

// ptyResizeLoop informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(_ context.Context, ptyC *PtyClient) error {
	// TODO: resize windows pty
	winSize, err := getPtySize(os.Stdin)
	if err != nil {
		return fmt.Errorf("failed to obtain window size: %v", err)
	}
	if err = ptyC.SetPtySize(winSize); err != nil {
		return fmt.Errorf("failed to set remote window size: %v", err)
	}
	return nil
}

// getPtySize obtains the size of the local terminal.
func getPtySize(_ *os.File) (console.WinSize, error) {
	return console.Current().Size()
}
