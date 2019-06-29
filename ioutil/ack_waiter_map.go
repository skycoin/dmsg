package ioutil

import (
	"context"
	"crypto/rand"
	"io"
	"sync"
)

// Uint16AckWaiterMap implements acknowledgement-waiting logic (with uint16 sequences).
// It stores data in map instead of array.
type Uint16AckWaiterMap struct {
	nextSeq Uint16Seq
	waiters map[Uint16Seq]chan struct{}
	mx      sync.RWMutex
}

// NewUint16AckWaiterMap creates new Uint16AckWaiterMap.
func NewUint16AckWaiterMap() Uint16AckWaiterMap {
	return Uint16AckWaiterMap{
		waiters: make(map[Uint16Seq]chan struct{}),
	}
}

// RandSeq should only be run once on startup. It is not thread-safe.
func (w *Uint16AckWaiterMap) RandSeq() error {
	b := make([]byte, 2)
	if _, err := rand.Read(b); err != nil {
		return err
	}
	w.nextSeq = DecodeUint16Seq(b)
	return nil
}

func (w *Uint16AckWaiterMap) stopWaiter(seq Uint16Seq) {
	if waiter := w.waiters[seq]; waiter != nil {
		close(waiter)
		w.waiters[seq] = nil
	}
}

// StopAll stops all active waiters.
func (w *Uint16AckWaiterMap) StopAll() {
	w.mx.Lock()
	for seq := range w.waiters {
		w.stopWaiter(Uint16Seq(seq))
	}
	w.mx.Unlock()
}

// Wait performs the given action, and waits for given seq to be Done.
func (w *Uint16AckWaiterMap) Wait(ctx context.Context, action func(seq Uint16Seq) error) (err error) {
	ackCh := make(chan struct{}, 1)

	w.mx.Lock()
	seq := w.nextSeq
	w.nextSeq++
	w.waiters[seq] = ackCh
	w.mx.Unlock()

	if err = action(seq); err != nil {
		return err
	}

	select {
	case _, ok := <-ackCh:
		if !ok {
			// waiter stopped manually.
			err = io.ErrClosedPipe
		}
	case <-ctx.Done():
		err = ctx.Err()
	}

	w.mx.Lock()
	w.stopWaiter(seq)
	w.mx.Unlock()
	return err
}

// Done finishes given sequence.
func (w *Uint16AckWaiterMap) Done(seq Uint16Seq) {
	w.mx.RLock()
	select {
	case w.waiters[seq] <- struct{}{}:
	default:
	}
	w.mx.RUnlock()
}
