package dmsg

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/SkycoinProject/dmsg/cipher"
)

func GenKeyPair(t *testing.T, seed string) (cipher.PubKey, cipher.SecKey) {
	pk, sk, err := cipher.GenerateDeterministicKeyPair([]byte(seed))
	require.NoError(t, err)
	return pk, sk
}

//func AddListener(t *testing.T, porter *netutil.Porter, addr Addr) *Listener {
//	lis := newListener(addr)
//	ok, doneFn := porter.Reserve(addr.Port, lis)
//	lis.doneFunc = doneFn
//	require.True(t, ok)
//	return lis
//}
