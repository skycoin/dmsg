package dmsgpty

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"sync"
)

type UISession struct {
	id uuid.UUID
	hc *PtyClient
	vm *viewManager
}

func newViewManager(log logrus.FieldLogger, cacheSize int) *viewManager {
	return &viewManager{
		log:   log,
		cache: newViewCache(cacheSize),
	}
}

type viewManager struct {
	log   logrus.FieldLogger
	cache *viewCache
	views []net.Conn // views contains connections to web interface via websocket
	mux   sync.Mutex
}

func (vm *viewManager) AddView(view net.Conn) error {
	vm.mux.Lock()
	defer vm.mux.Unlock()
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
	wg := new(sync.WaitGroup)
	wg.Add(len(vm.views))

	for i := range vm.views {
		i := i
		go func() {
			results[i] = writeToView(vm.views[i], b)
			wg.Done()
		}()
	}

	if err := writeToView(vm.cache, b); err != nil {
		vm.log.WithError(err).Warn("failed to write to view cache")
	}

	wg.Wait()

	// remove and log about failing views.
	for i := len(results)-1; i >= 0; i-- {
		if err := results[i]; err != nil {
			vm.log.
				WithError(err).
				WithField("remote_addr", vm.views[i].RemoteAddr()).
				WithField("view_index", i).
				Error()
			vm.views = append(vm.views[:i], vm.views[i+1:]...)
		}
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
