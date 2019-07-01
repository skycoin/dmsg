package dmsg

import (
	"context"
	"math"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/skycoin/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"

	"github.com/skycoin/dmsg/cipher"
)

const (
	chanReadThreshold = time.Second * 5
)

type transportWithError struct {
	tr  *Transport
	err error
}

func BenchmarkNewClientConn(b *testing.B) {
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()

	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewClientConn(log, p1, pk1, pk2)
	}
}

func BenchmarkClientConn_getNextInitID_1(b *testing.B) {
	benchmarkClientConnGetNextInitID(b, 1)
}

func BenchmarkClientConn_getNextInitID_10(b *testing.B) {
	benchmarkClientConnGetNextInitID(b, 10)
}

func BenchmarkClientConn_getNextInitID_100(b *testing.B) {
	benchmarkClientConnGetNextInitID(b, 100)
}

func BenchmarkClientConn_getNextInitID_1000(b *testing.B) {
	benchmarkClientConnGetNextInitID(b, 1000)
}

func benchmarkClientConnGetNextInitID(b *testing.B, n int) {
	cc, _ := clientConnWithTps(n)
	ctx := context.TODO()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cc.getNextInitID(ctx); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkClientConn_getTp_1(b *testing.B) {
	benchmarkClientConnGetTp(b, 1)
}

func BenchmarkClientConn_getTp_10(b *testing.B) {
	benchmarkClientConnGetTp(b, 10)
}

func BenchmarkClientConn_getTp_100(b *testing.B) {
	benchmarkClientConnGetTp(b, 100)
}

func BenchmarkClientConn_getTp_1000(b *testing.B) {
	benchmarkClientConnGetTp(b, 1000)
}

func benchmarkClientConnGetTp(b *testing.B, n int) {
	cc, ids := clientConnWithTps(n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.getTp(ids[i%len(ids)])
	}
}

func clientConnWithTps(n int) (*ClientConn, []uint16) {
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()
	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	cc := NewClientConn(log, p1, pk1, pk2)
	ids := make([]uint16, 0, n)
	for i := 0; i < n; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		ids = append(ids, id)
		tp := NewTransport(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
	}

	return cc, ids
}

func BenchmarkClientConn_setTp(b *testing.B) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()
	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	cc := NewClientConn(log, p1, pk1, pk2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		tp := NewTransport(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
	}
}

func TestClient(t *testing.T) {
	logger := logging.MustGetLogger("dms_client")

	// Runs two ClientConn's and dials a transport from one to another.
	// Checks if states change properly and if closing of transport and connections works.
	t.Run("Two connections", func(t *testing.T) {
		p1, p2 := net.Pipe()
		p1, p2 = invertedIDConn{p1}, invertedIDConn{p2}

		var pk1, pk2 cipher.PubKey
		err := pk1.Set("024ec47420176680816e0406250e7156465e4531f5b26057c9f6297bb0303558c7")
		assert.NoError(t, err)
		err = pk2.Set("031b80cd5773143a39d940dc0710b93dcccc262a85108018a7a95ab9af734f8055")
		assert.NoError(t, err)

		conn1 := NewClientConn(logger, p1, pk1, pk2)
		conn2 := NewClientConn(logger, p2, pk2, pk1)

		ch1 := make(chan *Transport, AcceptBufferSize)
		ch2 := make(chan *Transport, AcceptBufferSize)

		ctx := context.TODO()

		go func() {
			_ = conn1.Serve(ctx, ch1) // nolint:errcheck
		}()

		go func() {
			_ = conn2.Serve(ctx, ch2) // nolint:errcheck
		}()

		conn1.mx.RLock()
		initID := conn1.nextInitID
		conn1.mx.RUnlock()
		_, ok := conn1.getTp(initID)
		assert.False(t, ok)

		tr1, err := conn1.DialTransport(ctx, pk2)
		assert.NoError(t, err)

		_, ok = conn1.getTp(initID)
		assert.True(t, ok)
		conn1.mx.RLock()
		newInitID := conn1.nextInitID
		conn1.mx.RUnlock()
		assert.Equal(t, initID+2, newInitID)

		err = tr1.Close()
		assert.NoError(t, err)

		err = conn1.Close()
		assert.NoError(t, err)

		err = conn2.Close()
		assert.NoError(t, err)

		assert.False(t, isDoneChannelOpen(conn1.done))
		assert.False(t, isDoneChannelOpen(conn2.done))
		assert.False(t, isDoneChannelOpen(tr1.done))
		assert.False(t, isReadChannelOpen(tr1.inCh))
	})

	// Runs four ClientConn's and dials two transports between them.
	// Checks if states change properly and if closing of transports and connections works.
	t.Run("Four connections", func(t *testing.T) {
		p1, p2 := net.Pipe()
		p1, p2 = invertedIDConn{p1}, invertedIDConn{p2}

		p3, p4 := net.Pipe()
		p3, p4 = invertedIDConn{p3}, invertedIDConn{p4}

		var pk1, pk2, pk3 cipher.PubKey
		err := pk1.Set("024ec47420176680816e0406250e7156465e4531f5b26057c9f6297bb0303558c7")
		assert.NoError(t, err)
		err = pk2.Set("031b80cd5773143a39d940dc0710b93dcccc262a85108018a7a95ab9af734f8055")
		assert.NoError(t, err)
		err = pk3.Set("035b57eef30b9a6be1effc2c3337a3a1ffedcd04ffbac6667cd822892cf56be24a")
		assert.NoError(t, err)

		conn1 := NewClientConn(logger, p1, pk1, pk2)
		conn2 := NewClientConn(logger, p2, pk2, pk1)
		conn3 := NewClientConn(logger, p3, pk2, pk3)
		conn4 := NewClientConn(logger, p4, pk3, pk2)

		conn2.setNextInitID(randID(false))
		conn4.setNextInitID(randID(false))

		ch1 := make(chan *Transport, AcceptBufferSize)
		ch2 := make(chan *Transport, AcceptBufferSize)
		ch3 := make(chan *Transport, AcceptBufferSize)
		ch4 := make(chan *Transport, AcceptBufferSize)

		ctx := context.TODO()

		go func() {
			_ = conn1.Serve(ctx, ch1) // nolint:errcheck
		}()

		go func() {
			_ = conn2.Serve(ctx, ch2) // nolint:errcheck
		}()

		go func() {
			_ = conn3.Serve(ctx, ch3) // nolint:errcheck
		}()

		go func() {
			_ = conn4.Serve(ctx, ch4) // nolint:errcheck
		}()

		conn1.mx.RLock()
		initID1 := conn1.nextInitID
		conn1.mx.RUnlock()

		_, ok := conn1.getTp(initID1)
		assert.False(t, ok)

		conn2.mx.RLock()
		initID2 := conn2.nextInitID
		conn2.mx.RUnlock()

		_, ok = conn2.getTp(initID2)
		assert.False(t, ok)

		conn3.mx.RLock()
		initID3 := conn3.nextInitID
		conn3.mx.RUnlock()

		_, ok = conn3.getTp(initID3)
		assert.False(t, ok)

		conn4.mx.RLock()
		initID4 := conn4.nextInitID
		conn4.mx.RUnlock()

		_, ok = conn4.getTp(initID4)
		assert.False(t, ok)

		trCh1 := make(chan transportWithError)
		trCh2 := make(chan transportWithError)

		go func() {
			tr, err := conn1.DialTransport(ctx, pk2)
			trCh1 <- transportWithError{
				tr:  tr,
				err: err,
			}
		}()

		go func() {
			tr, err := conn3.DialTransport(ctx, pk3)
			trCh2 <- transportWithError{
				tr:  tr,
				err: err,
			}
		}()

		twe1 := <-trCh1
		twe2 := <-trCh2

		tr1, err := twe1.tr, twe1.err
		assert.NoError(t, err)

		_, ok = conn1.getTp(initID1)
		assert.True(t, ok)
		conn1.mx.RLock()
		newInitID1 := conn1.nextInitID
		conn1.mx.RUnlock()
		assert.Equal(t, initID1+2, newInitID1)

		tr2, err := twe2.tr, twe2.err
		assert.NoError(t, err)

		_, ok = conn3.getTp(initID3)
		assert.True(t, ok)
		conn3.mx.RLock()
		newInitID3 := conn3.nextInitID
		conn3.mx.RUnlock()
		assert.Equal(t, initID3+2, newInitID3)

		errCh1 := make(chan error)
		errCh2 := make(chan error)
		errCh3 := make(chan error)
		errCh4 := make(chan error)

		go func() {
			errCh1 <- tr1.Close()
		}()

		go func() {
			errCh2 <- tr2.Close()
		}()

		err = <-errCh1
		assert.NoError(t, err)

		err = <-errCh2
		assert.NoError(t, err)

		go func() {
			errCh1 <- conn1.Close()
		}()

		go func() {
			errCh2 <- conn2.Close()
		}()

		go func() {
			errCh3 <- conn3.Close()
		}()

		go func() {
			errCh4 <- conn4.Close()
		}()

		err = <-errCh1
		assert.NoError(t, err)

		err = <-errCh2
		assert.NoError(t, err)

		err = <-errCh3
		assert.NoError(t, err)

		err = <-errCh4
		assert.NoError(t, err)

		assert.False(t, isDoneChannelOpen(conn1.done))
		assert.False(t, isDoneChannelOpen(conn3.done))
		assert.False(t, isDoneChannelOpen(tr1.done))
		assert.False(t, isReadChannelOpen(tr1.inCh))
		assert.False(t, isDoneChannelOpen(tr2.done))
		assert.False(t, isReadChannelOpen(tr2.inCh))
	})
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

// used so that we can get two 'ClientConn's directly communicating with one another.
type invertedIDConn struct {
	net.Conn
}

// Write ensures odd IDs turn even, and even IDs turn odd on write.
func (c invertedIDConn) Write(b []byte) (n int, err error) {
	frame := Frame(b)
	newFrame := MakeFrame(frame.Type(), frame.TpID()^1, frame.Pay())
	return c.Conn.Write(newFrame)
}
