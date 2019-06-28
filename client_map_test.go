package dmsg

import (
	"context"
	"math"
	"math/rand"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cipher"
)

// ClientConnMap represents a connection between a dmsg.Client and dmsg.Server from a client's perspective.
type ClientConnMap struct {
	log *logging.Logger

	net.Conn                // conn to dmsg server
	local     cipher.PubKey // local client's pk
	remoteSrv cipher.PubKey // dmsg server's public key

	// nextInitID keeps track of unused tp_ids to assign a future locally-initiated tp.
	// locally-initiated tps use an even tp_id between local and intermediary dms_server.
	nextInitID uint16

	// Transports: map of transports to remote dms_clients (key: tp_id, val: transport).
	tps map[uint16]*Transport
	mx  sync.RWMutex // to protect tps

	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup
}

// NewClientConn creates a new ClientConn.
func NewClientConnMap(log *logging.Logger, conn net.Conn, local, remote cipher.PubKey) *ClientConnMap {
	cc := &ClientConnMap{
		log:        log,
		Conn:       conn,
		local:      local,
		remoteSrv:  remote,
		nextInitID: randID(true),
		done:       make(chan struct{}),
		tps:        make(map[uint16]*Transport),
	}
	cc.wg.Add(1)
	return cc
}

func (c *ClientConnMap) getNextInitID(ctx context.Context) (uint16, error) {
	for {
		select {
		case <-c.done:
			return 0, ErrClientClosed
		case <-ctx.Done():
			return 0, ctx.Err()
		default:
			if ch := c.tps[c.nextInitID]; ch != nil && !ch.IsClosed() {
				c.nextInitID += 2
				continue
			}
			c.tps[c.nextInitID] = nil
			id := c.nextInitID
			c.nextInitID = id + 2
			return id, nil
		}
	}
}

func (c *ClientConnMap) setTp(tp *Transport) {
	c.mx.Lock()
	c.tps[tp.id] = tp
	c.mx.Unlock()
}

func (c *ClientConnMap) delTp(id uint16) {
	c.mx.Lock()
	c.tps[id] = nil
	c.mx.Unlock()
}

func (c *ClientConnMap) getTp(id uint16) (*Transport, bool) {
	c.mx.RLock()
	tp := c.tps[id]
	c.mx.RUnlock()
	ok := tp != nil && !tp.IsClosed()
	return tp, ok
}

func (c *ClientConnMap) close() (closed bool) {
	c.once.Do(func() {
		closed = true
		c.log.WithField("remoteServer", c.remoteSrv).Infoln("ClosingConnection")
		close(c.done)
		c.mx.Lock()
		for _, tp := range c.tps {
			if tp != nil {
				go tp.Close() //nolint:errcheck
			}
		}
		_ = c.Conn.Close() //nolint:errcheck
		c.mx.Unlock()
	})
	return closed
}

func BenchmarkNewClientConnMap(b *testing.B) {
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()

	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	for i := 0; i < b.N; i++ {
		NewClientConnMap(log, p1, pk1, pk2)
	}
}

func BenchmarkGetNextInitIDMap_1(b *testing.B) {
	benchmarkGetNextInitIDMap(b, 1)
}

func BenchmarkGetNextInitIDMap_10(b *testing.B) {
	benchmarkGetNextInitIDMap(b, 10)
}

func BenchmarkGetNextInitIDMap_100(b *testing.B) {
	benchmarkGetNextInitIDMap(b, 100)
}

func BenchmarkGetNextInitIDMap_1000(b *testing.B) {
	benchmarkGetNextInitIDMap(b, 1000)
}

func benchmarkGetNextInitIDMap(b *testing.B, n int) {
	cc, _ := clientConnMapWithTps(n)
	ctx := context.TODO()

	for i := 0; i < b.N; i++ {
		if _, err := cc.getNextInitID(ctx); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkGetTpMap_1(b *testing.B) {
	benchmarkGetTpMap(b, 1)
}

func BenchmarkGetTpMap_10(b *testing.B) {
	benchmarkGetTpMap(b, 10)
}

func BenchmarkGetTpMap_100(b *testing.B) {
	benchmarkGetTpMap(b, 100)
}

func BenchmarkGetTpMap_1000(b *testing.B) {
	benchmarkGetTpMap(b, 1000)
}

func benchmarkGetTpMap(b *testing.B, n int) {
	cc, ids := clientConnMapWithTps(n)

	for i := 0; i < b.N; i++ {
		cc.getTp(ids[i%len(ids)])
	}
}

func BenchmarkSetTpMap(b *testing.B) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()
	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	cc := NewClientConnMap(log, p1, pk1, pk2)

	for i := 0; i < b.N; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		tp := NewTransport(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
	}
}

func BenchmarkCloseTpMap_1(b *testing.B) {
	benchmarkCloseTpMap(b, 1)
}

func BenchmarkCloseTpMap_10(b *testing.B) {
	benchmarkCloseTpMap(b, 10)
}

func BenchmarkCloseTpMap_100(b *testing.B) {
	benchmarkCloseTpMap(b, 100)
}

func BenchmarkCloseTpMap_1000(b *testing.B) {
	benchmarkCloseTpMap(b, 1000)
}

func benchmarkCloseTpMap(b *testing.B, n int) {
	cc, _ := clientConnMapWithTps(n)

	for i := 0; i < b.N; i++ {
		cc.close()
	}
}

func clientConnMapWithTps(n int) (*ClientConnMap, []uint16) {
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()
	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	cc := NewClientConnMap(log, p1, pk1, pk2)
	ids := make([]uint16, 0, n)
	for i := 0; i < n; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		ids = append(ids, id)
		tp := NewTransport(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
	}

	return cc, ids
}
