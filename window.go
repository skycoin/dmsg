package dmsg

import (
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"sync"
)

type LocalWindow struct {
	r   int           // remaining window (in bytes)
	max int           // max possible window (in bytes)
	buf net.Buffers   // buffer for unread bytes
	ch  chan struct{} // indicator for new data in 'buf'
	mx  sync.Mutex    // race protection
}

func NewLocalWindow(size int) *LocalWindow {
	return &LocalWindow{
		r:   size,
		max: size,
		buf: make(net.Buffers, 0, 255),
		ch:  make(chan struct{}, 1),
	}
}

func (lw *LocalWindow) Max() int {
	return lw.max
}

func (lw *LocalWindow) Enqueue(p []byte, tpDone chan struct{}) error {
	lw.mx.Lock()
	defer lw.mx.Unlock()

	// Offset local window.
	// If the length of the FWD payload exceeds local window, then the remote client is not respecting our
	// advertised window size.
	if lw.r -= len(p); lw.r < 0 || lw.r > lw.max {
		return errors.New("failed to enqueue local window: remote is not respecting advertised window size")
	}

	lw.buf = append(lw.buf, p)
	if !isDone(tpDone) {
		select {
		case lw.ch <- struct{}{}:
		default:
		}
	}

	return nil
}

func (lw *LocalWindow) Read(p []byte, tpDone <-chan struct{}, sendAck func(uint16)) (n int, err error) {
	// return if 'p' has 0 len
	if len(p) == 0 {
		lw.mx.Lock()
		if isDone(tpDone) && lw.r == lw.max {
			err = io.EOF
		}
		lw.mx.Unlock()
		return
	}

	for {
		// We limit the reader so that ACK frames has an 'window_offset' field that is in scope.
		r := io.LimitReader(&lw.buf, math.MaxUint16)

		lw.mx.Lock()
		if n, err = r.Read(p); n > 0 {
			// increase local window and send ACK.
			if lw.r += n; lw.r < 0 || lw.r > lw.max {
				lw.mx.Unlock()
				panic(fmt.Errorf("bug: local window size became invalid after read: remaining(%d) min(%d) max(%d)",
					lw.r, 0, lw.max))
			}

			if !isDone(tpDone) {
				go sendAck(uint16(n))
				err = nil
			}
			lw.mx.Unlock()
			return n, err
		}
		lw.mx.Unlock()

		if _, ok := <-lw.ch; !ok {
			return n, err
		}
	}
}

func (lw *LocalWindow) Close() error {
	if lw == nil {
		return nil
	}
	lw.mx.Lock()
	close(lw.ch)
	lw.mx.Unlock()
	return nil
}

type RemoteWindow struct {
	r   int           // remaining window (in bytes)
	max int           // max possible window (in bytes)
	ch  chan struct{} // blocks writes until remote window clears up
	wMx sync.Mutex    // ensures only one write can happen at one time
	mx  sync.Mutex    // race protection
}

func NewRemoteWindow(size int) *RemoteWindow {
	return &RemoteWindow{
		r:   size,
		max: size,
		ch:  make(chan struct{}, 1),
	}
}

// Grow should be triggered when we receive a remote ACK to grow our record of the remote window.
func (rw *RemoteWindow) Grow(n int, tpDone <-chan struct{}) error {
	rw.mx.Lock()
	defer rw.mx.Unlock()

	// grow remaining window
	if rw.r += n; rw.r < 0 || rw.r > rw.max {
		return fmt.Errorf("local record of remote window has become invalid: remaning(%d) min(%d) max(%d)", rw.r, 0, rw.max)
	}
	fmt.Printf("RemoteWindow.Grow: rw.r(%d) m(%d)\n", rw.r, n)

	if !isDone(tpDone) {
		select {
		case rw.ch <- struct{}{}:
		default:
		}
	}

	return nil
}

func (rw *RemoteWindow) Write(p []byte, sendFwd func([]byte) error) (n int, err error) {
	rw.wMx.Lock()
	defer rw.wMx.Unlock()

	for lastN, r := 0, rw.remaining(); len(p) > 0 && err == nil; n = n+lastN {
		// if remaining window has len 0, wait until it opens up
		if r == 0 {
			if _, ok := <-rw.ch; !ok {
				return 0, io.ErrClosedPipe
			}
			continue
		}

		// write FWD frame and update 'p' and 'r'
		lastN, err = rw.write(&p, &r, sendFwd)
	}
	return
}

func (rw *RemoteWindow) write(p *[]byte, r *int, sendFwd func([]byte) error) (n int, err error) {
	n = len(*p)

	// ensure written payload does not surpass remaining remote window size or maximum allowed FWD payload size
	if n > *r {
		n = *r
	}
	if n > tpBufCap {
		n = tpBufCap
	}

	// write FWD and remove written portion of 'p'
	if err := sendFwd((*p)[:n]); err != nil {
		return 0, err
	}
	*p = (*p)[n:]

	// shrink remaining remote window
	*r, err = rw.shrink(n)
	return n, err
}

func (rw *RemoteWindow) shrink(dec int) (int, error) {
	rw.mx.Lock()
	defer rw.mx.Unlock()
	if rw.r -= dec; rw.r < 0 || rw.r > rw.max {
		return dec, fmt.Errorf("local record of remote window has become invalid: remaning(%d) min(%d) max(%d)", rw.r, 0, rw.max)
	}
	return rw.r, nil
}

func (rw *RemoteWindow) remaining() int {
	rw.mx.Lock()
	defer rw.mx.Unlock()
	return rw.r
}

func (rw *RemoteWindow) Close() error {
	if rw == nil {
		return nil
	}
	rw.mx.Lock()
	close(rw.ch)
	rw.mx.Unlock()
	return nil
}

/*
	Helper functions.
*/

func isDone(done <-chan struct{}) bool {
	select {
	case <-done:
		return true
	default:
		return false
	}
}
