//+build !windows

package dmsgpty

import (
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Start runs a command with the given command name, args and optional window size.
func (s *Pty) Start(name string, args []string, size *pty.Winsize) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.pty != nil {
		return ErrPtyAlreadyRunning
	}

	cmd := exec.Command(name, args...) //nolint:gosec
	cmd.Env = os.Environ()

	f, err := pty.StartWithSize(cmd, size) //nolint:gosec
	if err != nil {
		return err
	}

	s.pty = f
	return nil
}

// SetPtySize sets the pty size.
func (s *Pty) SetPtySize(size *pty.Winsize) error {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return ErrPtyNotRunning
	}

	return pty.Setsize(s.pty, size)
}
