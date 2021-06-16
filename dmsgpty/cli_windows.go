//+build windows

package dmsgpty

import (
	"context"
	"fmt"
	"github.com/ActiveState/termtest/conpty"
	"io"
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

// servePty serves a pty connection via the dmsgpty-host.
func (cli *CLI) servePty(ctx context.Context, ptyC *PtyClient, cmd string, args []string) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cli.Log.
		WithField("cmd", fmt.Sprint(append([]string{cmd}, args...))).
		Infof("Executing...")

	f, err := conpty.InitTerminal(false)
	if err != nil {
		return err
	}
	defer f()

	if err := ptyC.Start(cmd, args...); err != nil {
		return fmt.Errorf("failed to start command on pty: %v", err)
	}

	// Window resize loop.
	go func() {
		defer cancel()
		if err := ptyResizeLoop(ctx, ptyC); err != nil {
			cli.Log.
				WithError(err).
				Warn("Window resize loop closed with error.")
		}
	}()

	// Write loop.
	go func() {
		defer cancel()
		_, _ = io.Copy(ptyC, os.Stdin) //nolint:errcheck
	}()

	// Read loop.
	if _, err := io.Copy(os.Stdout, ptyC); err != nil {
		cli.Log.
			WithError(err).
			Error("Read loop closed with error.")
	}

	return nil
}
