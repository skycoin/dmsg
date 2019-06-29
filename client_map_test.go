package dmsg

import (
	"context"
	"math"
	"math/rand"
	"net"
	"testing"
	"time"

	"github.com/skycoin/skycoin/src/util/logging"

	"github.com/skycoin/dmsg/cipher"
)

func BenchmarkNewClientConnMap(b *testing.B) {
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()

	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewClientConnMap(log, p1, pk1, pk2)
	}
}

func BenchmarkClientConnMap_getNextInitID_1(b *testing.B) {
	benchmarkClientConnMapGetNextInitID(b, 1)
}

func BenchmarkClientConnMap_getNextInitID_10(b *testing.B) {
	benchmarkClientConnMapGetNextInitID(b, 10)
}

func BenchmarkClientConnMap_getNextInitID_100(b *testing.B) {
	benchmarkClientConnMapGetNextInitID(b, 100)
}

func BenchmarkClientConnMap_getNextInitID_1000(b *testing.B) {
	benchmarkClientConnMapGetNextInitID(b, 1000)
}

func benchmarkClientConnMapGetNextInitID(b *testing.B, n int) {
	cc, _ := clientConnMapWithTps(n)
	ctx := context.TODO()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := cc.getNextInitID(ctx); err != nil {
			b.Error(err)
		}
	}
}

func BenchmarkClientConnMap_getTpMap_1(b *testing.B) {
	benchmarkClientConnMapGetTpMap(b, 1)
}

func BenchmarkClientConnMap_getTpMap_10(b *testing.B) {
	benchmarkClientConnMapGetTpMap(b, 10)
}

func BenchmarkClientConnMap_getTpMap_100(b *testing.B) {
	benchmarkClientConnMapGetTpMap(b, 100)
}

func BenchmarkClientConnMap_getTpMap_1000(b *testing.B) {
	benchmarkClientConnMapGetTpMap(b, 1000)
}

func benchmarkClientConnMapGetTpMap(b *testing.B, n int) {
	cc, ids := clientConnMapWithTps(n)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cc.getTp(ids[i%len(ids)])
	}
}

func BenchmarkClientConnMap_setTp(b *testing.B) {
	rand := rand.New(rand.NewSource(time.Now().UnixNano()))
	log := logging.MustGetLogger("dmsg_test")

	p1, _ := net.Pipe()
	pk1, _ := cipher.GenerateKeyPair()
	pk2, _ := cipher.GenerateKeyPair()

	cc := NewClientConnMap(log, p1, pk1, pk2)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		id := uint16(rand.Intn(math.MaxUint16))
		tp := NewTransportMap(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
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
		tp := NewTransportMap(p1, log, cipher.PubKey{}, cipher.PubKey{}, id, cc.delTp)
		cc.setTp(tp)
	}

	return cc, ids
}
