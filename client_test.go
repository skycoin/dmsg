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
	logger := logging.MustGetLogger("dmsg_client")

	// Runs two ClientConn's and dials a transport from one to another.
	// Checks if states change properly and if closing of transport and connections works.
	t.Run("Two connections", func(t *testing.T) {
		p1, p2 := net.Pipe()
		p1, p2 = invertedIDConn{p1}, invertedIDConn{p2}

		pk1, _ := cipher.GenerateKeyPair()
		pk2, _ := cipher.GenerateKeyPair()

		conn1 := NewClientConn(logger, p1, pk1, pk2)
		conn2 := NewClientConn(logger, p2, pk2, pk1)

		ch1 := make(chan *Transport, AcceptBufferSize)
		ch2 := make(chan *Transport, AcceptBufferSize)

		ctx := context.TODO()

		serveErrCh1 := make(chan error, 1)
		go func() {
			serveErrCh1 <- conn1.Serve(ctx, ch1)
			close(serveErrCh1)
		}()

		serveErrCh2 := make(chan error, 1)
		go func() {
			serveErrCh2 <- conn2.Serve(ctx, ch2)
			close(serveErrCh2)
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

		assert.NoError(t, closeClosers(conn1, conn2))
		checkClientConnsClosed(t, conn1, conn2)

		assert.Error(t, errWithTimeout(serveErrCh1))
		assert.Error(t, errWithTimeout(serveErrCh2))

		assert.True(t, tr1.IsClosed())
	})

	// Runs four ClientConn's and dials two transports between them.
	// Checks if states change properly and if closing of transports and connections works.
	t.Run("Four connections", func(t *testing.T) {
		p1, p2 := net.Pipe()
		p1, p2 = invertedIDConn{p1}, invertedIDConn{p2}

		p3, p4 := net.Pipe()
		p3, p4 = invertedIDConn{p3}, invertedIDConn{p4}

		pk1, _ := cipher.GenerateKeyPair()
		pk2, _ := cipher.GenerateKeyPair()
		pk3, _ := cipher.GenerateKeyPair()

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

		serveErrCh1 := make(chan error, 1)
		go func() {
			serveErrCh1 <- conn1.Serve(ctx, ch1)
			close(serveErrCh1)
		}()

		serveErrCh2 := make(chan error, 1)
		go func() {
			serveErrCh2 <- conn2.Serve(ctx, ch2)
			close(serveErrCh2)
		}()

		serveErrCh3 := make(chan error, 1)
		go func() {
			serveErrCh3 <- conn3.Serve(ctx, ch3)
			close(serveErrCh3)
		}()

		serveErrCh4 := make(chan error, 1)
		go func() {
			serveErrCh4 <- conn4.Serve(ctx, ch4)
			close(serveErrCh4)
		}()

		initID1 := getNextInitID(conn1)
		_, ok := conn1.getTp(initID1)
		assert.False(t, ok)

		initID2 := getNextInitID(conn2)
		_, ok = conn2.getTp(initID2)
		assert.False(t, ok)

		initID3 := getNextInitID(conn3)
		_, ok = conn3.getTp(initID3)
		assert.False(t, ok)

		initID4 := getNextInitID(conn4)
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

		assert.NoError(t, closeClosers(tr1, tr2, conn1, conn2, conn3, conn4))
		checkTransportsClosed(t, tr1, tr2)
		checkClientConnsClosed(t, conn1, conn3)

		assert.Error(t, errWithTimeout(serveErrCh1))
		assert.Error(t, errWithTimeout(serveErrCh2))
		assert.Error(t, errWithTimeout(serveErrCh3))
		assert.Error(t, errWithTimeout(serveErrCh4))
	})
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
