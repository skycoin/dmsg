// Package noise pkg/noise/dh.go
package noise

import (
	"fmt"
	"io"

	"github.com/skycoin/noise"
	"github.com/skycoin/skycoin/src/cipher"
)

const keypairPoolSize = 64

// keypairPool holds pre-generated ephemeral keypairs for noise handshakes.
// secp256k1 key generation is expensive (EC multiply + validation), so we
// generate them in the background and serve them from a buffered channel.
var keypairPool = func() chan noise.DHKey {
	ch := make(chan noise.DHKey, keypairPoolSize)
	go func() {
		for {
			pk, sk := cipher.GenerateKeyPair()
			ch <- noise.DHKey{
				Private: sk[:],
				Public:  pk[:],
			}
		}
	}()
	return ch
}()

// Secp256k1 implements `noise.DHFunc`.
type Secp256k1 struct{}

// GenerateKeypair helps to implement `noise.DHFunc`.
func (Secp256k1) GenerateKeypair(_ io.Reader) (noise.DHKey, error) {
	return <-keypairPool, nil
}

// DH helps to implement `noise.DHFunc`.
func (Secp256k1) DH(sk, pk []byte) []byte {
	pubKey, err := cipher.NewPubKey(pk)
	if err != nil {
		panic(fmt.Sprintf("noise DH: invalid public key: %v", err))
	}
	secKey, err := cipher.NewSecKey(sk)
	if err != nil {
		panic(fmt.Sprintf("noise DH: invalid secret key: %v", err))
	}
	ecdh, err := cipher.ECDH(pubKey, secKey)
	if err != nil {
		panic(fmt.Sprintf("noise DH: ECDH failed: %v", err))
	}
	return append(ecdh, byte(0))
}

// DHLen helps to implement `noise.DHFunc`.
func (Secp256k1) DHLen() int {
	return 33
}

// DHName helps to implement `noise.DHFunc`.
func (Secp256k1) DHName() string {
	return "Secp256k1"
}
