//+build windows

package dmsgpty

import (
	"os"
	"os/exec"

	"github.com/containerd/console"
)

// Start runs a command with the given command name, args and optional window size.
func (s *Pty) Start(name string, args []string, c console.Console) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.pty != nil {
		return ErrPtyAlreadyRunning
	}

	cmd := exec.Command(name, args...) //nolint:gosec
	cmd.Env = os.Environ()

	s.pty = os.NewFile(c.Fd(), "winconsole")
	return nil
}

// SetPtySize sets the pty size.
func (s *Pty) SetPtySize(size console.WinSize) error {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return ErrPtyNotRunning
	}

	c, err := console.ConsoleFromFile(s.pty)
	if err != nil {
		return err
	}
	return c.Resize(size)
}
