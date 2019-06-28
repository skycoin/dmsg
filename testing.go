package dmsg

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	noDelay           = time.Duration(0)
	smallDelay        = 100 * time.Millisecond
	chanReadThreshold = 5 * time.Second
	testTimeout       = 5 * time.Second
)

func closeClosers(closers ...io.Closer) error {
	for _, closer := range closers {
		if err := closer.Close(); err != nil {
			return err
		}
	}
	return nil
}

type connCounter interface {
	connCount() int
}

func checkConnCount(t *testing.T, cc connCounter, count int, delay time.Duration) {
	require.NoError(t, testWithTimeout(delay, func() error {
		if cc.connCount() != count {
			return fmt.Errorf("connCount equals to %d, want %d", cc.connCount(), count)
		}
		return nil
	}))
}

func checkTransportsClosed(t *testing.T, transports ...*Transport) {
	for _, transport := range transports {
		assert.False(t, isDoneChannelOpen(transport.done))
		assert.False(t, isReadChannelOpen(transport.inCh))
	}
}

func checkClientConnsClosed(t *testing.T, conns ...*ClientConn) {
	for _, conn := range conns {
		assert.False(t, isDoneChannelOpen(conn.done))
	}
}

func catch(err error) {
	if err != nil {
		panic(err)
	}
}

// intended to test some func of `func() error` signature with a given timeout.
// Exceeding timeout results in error.
func testWithTimeout(timeout time.Duration, run func() error) error {
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	for {
		if err := run(); err != nil {
			select {
			case <-timer.C:
				return err
			default:
				time.Sleep(testTimeout)
				continue
			}
		}

		return nil
	}
}

func isDoneChannelOpen(ch chan struct{}) bool {
	select {
	case _, ok := <-ch:
		return ok
	case <-time.After(chanReadThreshold):
		return false
	}
}

func isReadChannelOpen(ch chan Frame) bool {
	select {
	case _, ok := <-ch:
		return ok
	case <-time.After(chanReadThreshold):
		return false
	}
}

func getNextInitID(conn *ClientConn) uint16 {
	conn.mx.Lock()
	defer conn.mx.Unlock()

	return conn.nextInitID
}

func getNextRespID(conn *ServerConn) uint16 {
	conn.mx.Lock()
	defer conn.mx.Unlock()

	return conn.nextRespID
}
