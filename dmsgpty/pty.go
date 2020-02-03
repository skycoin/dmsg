package dmsgpty

import (
	"errors"
	"github.com/creack/pty"
	"os"
	"os/exec"
	"sync"
)

// Pty errors.
var (
	ErrPtyAlreadyRunning = errors.New("a pty session is already running")
	ErrPtyNotRunning     = errors.New("no active pty session")
)

type Pty struct {
	pty *os.File
	mx  sync.RWMutex
}

func NewPty() *Pty {
	return new(Pty)
}

func (s *Pty) Start(name string, args []string, size *pty.Winsize) error {
	s.mx.Lock()
	defer s.mx.Unlock()

	if s.pty != nil {
		return ErrPtyAlreadyRunning
	}

	cmd := exec.Command(name, args...)
	cmd.Env = append(
		os.Environ(),
	)

	f, err := pty.StartWithSize(cmd, size) //nolint:gosec
	if err != nil {
		return err
	}

	s.pty = f
	return nil
}

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

func (s *Pty) Read(b []byte) (int, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return 0, ErrPtyNotRunning
	}

	return s.pty.Read(b)
}

func (s *Pty) Write(b []byte) (int, error) {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return 0, ErrPtyNotRunning
	}

	return s.pty.Write(b)
}

func (s *Pty) SetPtySize(size *pty.Winsize) error {
	s.mx.RLock()
	defer s.mx.RUnlock()

	if s.pty == nil {
		return ErrPtyNotRunning
	}

	return pty.Setsize(s.pty, size)
}
