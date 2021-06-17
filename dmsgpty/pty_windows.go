//+build windows

package dmsgpty

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"syscall"

	"github.com/ActiveState/termtest/conpty"
	"golang.org/x/sys/windows"
)

// Pty errors.
var (
	ErrPtyAlreadyRunning = errors.New("a pty session is already running")
	ErrPtyNotRunning     = errors.New("no active pty session")
)

// Pty runs a local pty.
type Pty struct {
	pty *conpty.ConPty
	mx  sync.RWMutex
}

// NewPty creates a new Pty.
func NewPty() *Pty {
	return new(Pty)
}

// Stop stops the running command and closes the pty.
func (s *Pty) Stop() error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.pty == nil {
		return ErrPtyNotRunning
	}

	err := s.pty.Close()
	s.pty = nil
	return err
}

// Read reads any stdout or stderr outputs from the pty.
func (s *Pty) Read(b []byte) (int, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return 0, ErrPtyNotRunning
	}

	return s.pty.OutPipe().Read(b)
}

// Write writes to the stdin of the pty.
func (s *Pty) Write(b []byte) (int, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return 0, ErrPtyNotRunning
	}

	res, err := s.pty.Write(b)
	return int(res), err
}

// Start runs a command with the given command name, args and optional window size.
func (s *Pty) Start(name string, args []string, size WinSizer) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	var sz *windows.Coord

	if s.pty != nil {
		return ErrPtyAlreadyRunning
	}

	var err error

	if size == nil {
		sz, err = getSize()
		if err != nil {
			return err
		}
	} else {
		sz = &windows.Coord{
			X: int16(size.Width()),
			Y: int16(size.Height()),
		}
	}

	pty, err := conpty.New(
		sz.X, sz.Y,
	)
	if err != nil {
		return err
	}

	pid, _, err := pty.Spawn(
		name,
		args,
		&syscall.ProcAttr{
			Env: os.Environ(),
		},
	)

	if err != nil {
		return err
	}

	fmt.Printf("starting process with pid %d \n", pid)

	s.pty = pty
	return nil
}

// SetPtySize sets the pty size.
func (s *Pty) SetPtySize(size WinSizer) error {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return ErrPtyNotRunning
	}

	return s.pty.Resize(size.Width(), size.Height())
}
