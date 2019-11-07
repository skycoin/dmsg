package dmsg

import (
	"math"
	"math/rand"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
)

func TestLocalWindow(t *testing.T) {
	const max = math.MaxUint16

	var (
		expR   int           // expected remaining size of the local window
		tpDone chan struct{} // emulate done chan of originating dmsg.Stream
		lw     *LocalWindow
	)

	reset := func() {
		expR = max
		tpDone = make(chan struct{})

		assert.NoError(t, lw.Close())
		lw = NewLocalWindow(max)
	}

	reset()

	t.Run("max_window_size_is_enforced", func(t *testing.T) {
		defer reset()

		for {
			n := rand.Intn(max / 10)
			expR -= n

			err := lw.Enqueue(make([]byte, n), tpDone)
			if expR < 0 {
				require.Error(t, err)
				break
			}
			require.NoError(t, err)
		}
	})

	t.Run("enqueued_bytes_can_be_read", func(t *testing.T) {
		defer reset()

		var allBytes []byte

		// write until window maxes out
		for {
			n := rand.Intn(max / 10)
			if expR-n < 0 {
				break
			}
			expR -= n

			b := cipher.RandByte(n)
			allBytes = append(allBytes, b...)

			require.NoError(t, lw.Enqueue(b, tpDone))
		}

		var readBytes []byte
		var acked uint64
		ackWg := new(sync.WaitGroup)

		// read all bytes
		for {
			b := make([]byte, rand.Intn(max/10))
			ackWg.Add(1)
			n, err := lw.Read(b, tpDone, func(u uint16) {
				atomic.AddUint64(&acked, uint64(u))
				ackWg.Done()
			})
			require.NoError(t, err)
			if readBytes = append(readBytes, b[:n]...); len(readBytes) == len(allBytes) {
				break
			}
		}

		ackWg.Wait()
		require.Equal(t, allBytes, readBytes)
		require.Equal(t, len(readBytes), int(acked))
	})
}

func TestRemoteWindow(t *testing.T) {
	const max = math.MaxUint16

	var (
		expR   int
		tpDone chan struct{}
		rw     *RemoteWindow
	)

	reset := func() {
		expR = max
		tpDone = make(chan struct{})

		assert.NoError(t, rw.Close())
		rw = NewRemoteWindow(max)
	}

	reset()

	t.Run("write", func(t *testing.T) {
		defer reset()

		var expBytes []byte
		var writtenBytes []byte
		var writtenN int

		for {
			n := rand.Intn(max / 10)
			if expR-n < 0 {
				break
			}
			expR -= n

			b := cipher.RandByte(n)
			expBytes = append(expBytes, b...)

			wn, err := rw.Write(b, func(b []byte) error {
				writtenBytes = append(writtenBytes, b...)
				return nil
			})
			writtenN += wn
			require.NoError(t, err)
		}

		require.Equal(t, len(writtenBytes), writtenN)
		require.Equal(t, expBytes, writtenBytes)
	})

	t.Run("write_max_payload", func(t *testing.T) {
		defer reset()

		b := cipher.RandByte(maxFwdPayLen)
		n, err := rw.Write(b, func(p []byte) error {
			require.Equal(t, b, p)
			return nil
		})
		require.NoError(t, err)
		require.Equal(t, maxFwdPayLen, n)

		// block further writes until remote window grows.

		wrote := make(chan struct{})
		pay := []byte("this is a payload!")

		var writtenPay []byte
		var writtenN int
		var writtenErr error

		go func() {
			writtenN, writtenErr = rw.Write(pay, func(p []byte) error {
				writtenPay = append(writtenPay, p...)
				return nil
			})
			close(wrote)
		}()

		time.Sleep(time.Millisecond * 200)
		require.False(t, isDone(wrote))

		require.NoError(t, rw.Grow(len(pay)-1, tpDone))

		time.Sleep(time.Millisecond * 200)
		require.False(t, isDone(wrote))

		require.NoError(t, rw.Grow(1, tpDone))

		time.Sleep(time.Millisecond * 200)
		require.True(t, isDone(wrote))

		require.Equal(t, pay, writtenPay)
		require.Equal(t, len(pay), writtenN)
		require.NoError(t, writtenErr)
	})

	close(tpDone)
}
