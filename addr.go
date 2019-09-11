package dmsg

import (
	"fmt"
	"math"
	"sync"

	"github.com/skycoin/dmsg/cipher"
)

// Addr implements net.Addr for skywire addresses.
type Addr struct {
	PK   cipher.PubKey `json:"public_key"`
	Port uint16        `json:"port"`
}

// Network returns "dmsg"
func (Addr) Network() string {
	return Type
}

// String returns public key and port of node split by colon.
func (a Addr) String() string {
	if a.Port == 0 {
		return fmt.Sprintf("%s:~", a.PK)
	}
	return fmt.Sprintf("%s:%d", a.PK, a.Port)
}

type ClientID uint32

type ClientCIDLinker interface {
	PubKey(id ClientID) (cipher.PubKey, bool)
	ClientID(pk cipher.PubKey) (ClientID, bool)
	Remove(pk cipher.PubKey)
}

type ServerCIDLinker interface {
}

type CIDLinker struct {
	pks map[ClientID]cipher.PubKey
	ids map[cipher.PubKey]ClientID
	nxt ClientID // next client ID.
	mx  sync.Mutex
}

func (l *CIDLinker) GetOrSet(pk cipher.PubKey) (ClientID, bool) {
	l.mx.Lock()
	defer l.mx.Unlock()

	id, ok := l.ids[pk]
	if !ok {
		if len(l.pks) >= math.MaxUint32 {
			return 0, false
		}
		for {
			id = l.nxt
			l.nxt++
			if _, ok := l.pks[id]; !ok {
				break
			}
		}

		l.pks[id] = pk
		l.ids[pk] = id
	}

	return id, true
}

func (l *CIDLinker) PubKey(id ClientID) (cipher.PubKey, bool) {
	l.mx.Lock()
	defer l.mx.Unlock()

	pk, ok := l.pks[id]
	return pk, ok
}

func (l *CIDLinker) ClientID(pk cipher.PubKey) (ClientID, bool) {
	l.mx.Lock()
	defer l.mx.Unlock()

	id, ok := l.ids[pk]
	return id, ok
}

func (l *CIDLinker) Remove(pk cipher.PubKey) {
	l.mx.Lock()
	defer l.mx.Unlock()

	id, ok := l.ids[pk]
	if !ok {
		return
	}
	delete(l.pks, id)
	delete(l.ids, pk)
}
