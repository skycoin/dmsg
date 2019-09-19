package dmsg

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
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
		NewClientConn(log, newPortManager(pk1), p1, pk1, pk2)
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

	cc := NewClientConn(log, newPortManager(pk1), p1, pk1, pk2)
	ids := make([]uint16, 0, n)
	for i := 0; i < n; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		ids = append(ids, id)
		tp := NewTransport(p1, log, Addr{}, Addr{}, id, func() { cc.delTp(id) })
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

	cc := NewClientConn(log, newPortManager(pk1), p1, pk1, pk2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		tp := NewTransport(p1, log, Addr{}, Addr{}, id, func() { cc.delTp(id) })
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

		pm1 := newPortManager(pk1)
		pm2 := newPortManager(pk2)

		conn1 := NewClientConn(logger, pm1, p1, pk1, pk2)
		conn2 := NewClientConn(logger, pm2, p2, pk2, pk1)

		lis1, ok := conn1.pm.NewListener(port)
		require.True(t, ok)
		require.Equal(t, Addr{pk1, port}, lis1.addr)

		lis2, ok := conn2.pm.NewListener(port)
		require.True(t, ok)
		require.Equal(t, Addr{pk2, port}, lis2.addr)

		fmt.Println("Created zstuf")

		ctx := context.TODO()

		serveErrCh1 := make(chan error, 1)
		go func() {
			serveErrCh1 <- conn1.Serve(ctx)
			close(serveErrCh1)
		}()

		serveErrCh2 := make(chan error, 1)
		go func() {
			serveErrCh2 <- conn2.Serve(ctx)
			close(serveErrCh2)
		}()

		conn1.mx.RLock()
		initID := conn1.nextInitID
		conn1.mx.RUnlock()
		_, ok = conn1.getTp(initID)
		assert.False(t, ok)

		fmt.Println("dialing...")

		tr1, err := conn1.DialTransport(ctx, pk2, port)
		require.NoError(t, err)

		fmt.Println("Dialed")

		_, ok = conn1.getTp(initID)
		assert.True(t, ok)
		conn1.mx.RLock()
		newInitID := conn1.nextInitID
		conn1.mx.RUnlock()
		assert.Equal(t, initID+2, newInitID)

		assert.NoError(t, closeClosers(conn1, conn2, pm1, pm2))
		checkClientConnsClosed(t, conn1, conn2)

		assert.Error(t, errWithTimeout(serveErrCh1))
		assert.Error(t, errWithTimeout(serveErrCh2))

		assert.True(t, tr1.IsClosed())
	})

	// Runs four ClientConn's and dials two transports between them.
	// Checks if states change properly and if closing of transports and connections works.
	t.Run("Four connections", func(t *testing.T) {
		pipe1, pipe2 := net.Pipe()
		pipe1, pipe2 = invertedIDConn{pipe1}, invertedIDConn{pipe2}

		pipe3, pipe4 := net.Pipe()
		pipe3, pipe4 = invertedIDConn{pipe3}, invertedIDConn{pipe4}

		pk1, _ := cipher.GenerateKeyPair()
		pk2, _ := cipher.GenerateKeyPair()
		pk3, _ := cipher.GenerateKeyPair()

		pm1 := newPortManager(pk1)
		pm2 := newPortManager(pk2)
		pm3 := newPortManager(pk3)

		conn1 := NewClientConn(logger, pm1, pipe1, pk1, pk2)
		conn2 := NewClientConn(logger, pm2, pipe2, pk2, pk1)
		conn3 := NewClientConn(logger, pm2, pipe3, pk2, pk3)
		conn4 := NewClientConn(logger, pm3, pipe4, pk3, pk2)

		conn2.setNextInitID(randID(false))
		conn4.setNextInitID(randID(false))

		conn1.pm.NewListener(port)
		conn2.pm.NewListener(port)
		conn3.pm.NewListener(port)
		conn4.pm.NewListener(port)

		ctx := context.TODO()

		serveErrCh1 := make(chan error, 1)
		go func() {
			serveErrCh1 <- conn1.Serve(ctx)
			close(serveErrCh1)
		}()

		serveErrCh2 := make(chan error, 1)
		go func() {
			serveErrCh2 <- conn2.Serve(ctx)
			close(serveErrCh2)
		}()

		serveErrCh3 := make(chan error, 1)
		go func() {
			serveErrCh3 <- conn3.Serve(ctx)
			close(serveErrCh3)
		}()

		serveErrCh4 := make(chan error, 1)
		go func() {
			serveErrCh4 <- conn4.Serve(ctx)
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
			tr, err := conn1.DialTransport(ctx, pk2, port)
			trCh1 <- transportWithError{
				tr:  tr,
				err: err,
			}
		}()

		go func() {
			tr, err := conn3.DialTransport(ctx, pk3, port)
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

		assert.NoError(t, closeClosers(tr1, tr2, conn1, conn2, conn3, conn4, pm1, pm2, pm3))
		checkTransportsClosed(t, tr1, tr2)
		checkClientConnsClosed(t, conn1, conn3)

		assert.Error(t, errWithTimeout(serveErrCh1))
		assert.Error(t, errWithTimeout(serveErrCh2))
		assert.Error(t, errWithTimeout(serveErrCh3))
		assert.Error(t, errWithTimeout(serveErrCh4))
	})

	// After a transport is established, attempt and single write and close.
	// The reading edge should read the message correctly.
	t.Run("close_tp_after_single_write", func(t *testing.T) {
		p1, p2 := net.Pipe()
		p1, p2 = invertedIDConn{p1}, invertedIDConn{p2}

		pk1, _ := cipher.GenerateKeyPair()
		pk2, _ := cipher.GenerateKeyPair()

		pm1 := newPortManager(pk1)
		defer func() { require.NoError(t, pm1.Close()) }()

		pm2 := newPortManager(pk2)
		defer func() { require.NoError(t, pm2.Close()) }()

		conn1 := NewClientConn(logging.MustGetLogger("conn1"), pm1, p1, pk1, pk2)
		conn1.pm.NewListener(port)

		serveErrCh1 := make(chan error, 1)
		go func() {
			serveErrCh1 <- conn1.Serve(context.TODO())
			close(serveErrCh1)
		}()
		defer func() { require.NoError(t, conn1.Close()) }()

		conn2 := NewClientConn(logging.MustGetLogger("conn2"), pm2, p2, pk2, pk1)
		conn2.pm.NewListener(port)

		serveErrCh2 := make(chan error, 1)
		go func() {
			serveErrCh2 <- conn2.Serve(context.TODO())
			close(serveErrCh2)
		}()
		defer func() { require.NoError(t, conn2.Close()) }()

		tp1, err := conn1.DialTransport(context.TODO(), pk2, port)
		require.NoError(t, err)
		defer func() { require.NoError(t, tp1.Close()) }()

		// TODO(evanlinjin): Fix this test.
		//tp2, ok := <-ch2
		//require.True(t, ok)
		//defer func() { require.NoError(t, tp2.Close()) }()
		//
		//for i, count := range []int{75, 3, 75 - 3} {
		//	if i%3 == 0 {
		//		write := cipher.RandByte(count)
		//		n, err := tp1.Write(write)
		//
		//		require.NoError(t, err)
		//		require.Equal(t, count, n)
		//	} else {
		//		read := make([]byte, count)
		//		n, err := io.ReadFull(tp2, read)
		//		require.NoError(t, err)
		//		require.Equal(t, count, n)
		//	}
		//}
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
