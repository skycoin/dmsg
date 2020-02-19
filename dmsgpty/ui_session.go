package dmsgpty

import (
	"errors"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

var (
	ErrSessionClosed = errors.New("session closed")
)

type UISession struct {
	id uuid.UUID
	hc *PtyClient
	vm *viewManager

	doneErr error
	doneN   int64
	done    chan struct{}
}

func NewUISession(id uuid.UUID, hc *PtyClient, cacheSize int) *UISession {
	s := &UISession{
		id:   id,
		hc:   hc,
		vm:   newViewManager(hc.log.WithField("ui_id", id.String()), cacheSize),
		done: make(chan struct{}),
	}
	go func() {
		s.doneN, s.doneErr = io.Copy(s.vm, s.hc)
		close(s.done)
	}()
	return s
}

func (s *UISession) Close() error {
	_ = s.hc.Close() //nolint:errcheck
	_ = s.vm.Close() //nolint:errcheck
	return nil
}

func (s *UISession) WaitClose() <-chan struct{} {
	return s.done
}

func (s *UISession) ServeView(view net.Conn) (int64, error) {
	if err := s.vm.AddView(view); err != nil {
		return 0, err
	}
	defer func() { _ = view.Close() }() //nolint:errcheck

	return io.Copy(s.hc, view)
}

func newViewManager(log logrus.FieldLogger, cacheSize int) *viewManager {
	return &viewManager{
		log:   log,
		cache: newViewCache(cacheSize),
	}
}

type viewManager struct {
	log    logrus.FieldLogger
	cache  *viewCache
	views  []net.Conn // views contains connections to web interface via websocket
	closed bool
	wWg    sync.WaitGroup // waits for write routines
	mux    sync.Mutex
}

func (vm *viewManager) Close() error {
	vm.mux.Lock()
	vm.closed = true
	for _, v := range vm.views {
		_ = v.Close() //nolint:errcheck
	}
	vm.mux.Unlock()
	return nil
}

func (vm *viewManager) AddView(view net.Conn) error {
	vm.mux.Lock()
	defer vm.mux.Unlock()

	if vm.closed {
		return ErrSessionClosed
	}

	if _, err := view.Write(vm.cache.Bytes()); err != nil {
		return fmt.Errorf("failed to write from cache: %v", err)
	}
	vm.views = append(vm.views, view)
	return nil
}

// Write implements io.Writer and writes to all views.
func (vm *viewManager) Write(b []byte) (int, error) {
	vm.mux.Lock()
	defer vm.mux.Unlock()

	// Record of write results.
	results := make([]error, len(vm.views))

	// Wait for all writes to complete.
	vm.wWg.Add(len(vm.views))

	for i := range vm.views {
		i := i
		go func() {
			results[i] = writeToView(vm.views[i], b)
			vm.wWg.Done()
		}()
	}

	if err := writeToView(vm.cache, b); err != nil {
		vm.log.WithError(err).Warn("failed to write to view cache")
	}

	vm.wWg.Wait()

	// remove and log about failing views.
	for i := len(results) - 1; i >= 0; i-- {
		if err := results[i]; err != nil {
			vm.log.
				WithError(err).
				WithField("remote_addr", vm.views[i].RemoteAddr()).
				WithField("view_index", i).
				Error()
			vm.views = append(vm.views[:i], vm.views[i+1:]...)
		}
	}

	if vm.closed {
		return len(b), ErrSessionClosed
	}
	return len(b), nil
}

func writeToView(view io.Writer, b []byte) error {
	n, err := view.Write(b)
	if err != nil {
		return err
	}
	if n < len(b) {
		return io.ErrShortWrite
	}
	return nil
}
