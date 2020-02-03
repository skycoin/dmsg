package dmsgpty

import (
	"context"
	"fmt"
	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/creack/pty"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
)

type CLI struct {
	Log  logrus.FieldLogger `json:"-"`
	Net  string `json:"cli_network"`
	Addr string `json:"cli_address"`
}

func (cli *CLI) setDefaults() {
	if cli.Log == nil {
		cli.Log = logging.MustGetLogger("dmsgpty-cli")
	}
	if cli.Net == "" {
		cli.Net = "unix"
	}
	if cli.Addr == "" {
		cli.Addr = "/tmp/dmsgpty.sock"
	}
}

func (cli *CLI) StartLocalPty(ctx context.Context, cmd string, args []string) error {
	cli.setDefaults()

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cli.Log.
		WithField("address", fmt.Sprintf("%s://%s", cli.Net, cli.Addr)).
		Infof("Requesting local pty ...")

	conn, err := net.Dial(cli.Net, cli.Addr)
	if err != nil {
		return fmt.Errorf("failed to connect to dmsgpty-host: %v", err)
	}

	if err := writeRequest(conn, "dmsgpty/pty"); err != nil {
		return fmt.Errorf("failed to initiate request to dmsgpty-host: %v", err)
	}

	ptyC := NewPtyClient(conn)
	cli.Log.
		WithField("cmd", fmt.Sprint(append([]string{cmd}, args...))).
		Infof("Executing...")

	// Set stdin to raw mode.
	oldState, err := terminal.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		cli.Log.
			WithError(err).
			Warn("Failed to set stdin to raw mode.")
	} else {
		defer func() {
			// Attempt to restore state.
			if err := terminal.Restore(int(os.Stdin.Fd()), oldState); err != nil {
				cli.Log.
					WithError(err).
					Error("Failed to restore original stdin state.")
			}
		}()
	}

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
			Error("read loop closed with error")
	}

	return nil
}

// Loop that informs the remote of changes to the local CLI terminal window size.
func ptyResizeLoop(ctx context.Context, ptyC *PtyClient) error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ch:
			winSize, err := getPtySize(os.Stdin)
			if err != nil {
				return fmt.Errorf("failed to obtain window size: %v", err)
			}
			if err := ptyC.SetPtySize(winSize); err != nil {
				return fmt.Errorf("failed to set remote window size: %v", err)
			}
		}
	}
}

// getPtySize obtains the size of the local terminal.
func getPtySize(t *os.File) (*pty.Winsize, error) {
	return pty.GetsizeFull(t)
}

