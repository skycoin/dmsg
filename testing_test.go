package dmsg

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
	"github.com/SkycoinProject/dmsg/netutil"
)

func GenKeyPair(t *testing.T, seed string) (cipher.PubKey, cipher.SecKey) {
	pk, sk, err := cipher.GenerateDeterministicKeyPair([]byte(seed))
	require.NoError(t, err)
	return pk, sk
}

func AddListener(t *testing.T, porter *netutil.Porter, addr Addr) *Listener {
	lis := newListener(addr)
	ok, doneFn := porter.Reserve(addr.Port, lis)
	lis.doneFunc = doneFn
	require.True(t, ok)
	return lis
}

func MakeGetter() (get SessionGetter, add func(ses *Session)) {
	var (
		sesMap = make(map[cipher.PubKey]*Session)
		mx     = new(sync.RWMutex)
	)
	get = func(pk cipher.PubKey) (*Session, bool) {
		mx.RLock()
		ses, ok := sesMap[pk]
		mx.RUnlock()
		return ses, ok
	}
	add = func(ses *Session) {
		mx.Lock()
		sesMap[ses.RemotePK()] = ses
		mx.Unlock()
	}
	return get, add
}