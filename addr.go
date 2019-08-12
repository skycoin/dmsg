package dmsg

import (
	"fmt"

	"github.com/skycoin/dmsg/cipher"
)

// Addr implements net.Addr for skywire addresses.
type Addr struct {
	pk   cipher.PubKey
	port *uint16
}

// Network returns "dmsg"
func (Addr) Network() string {
	return Type
}

// String returns public key and port of node split by colon.
func (a Addr) String() string {
	if a.port == nil {
		return a.pk.String()
	}
	return fmt.Sprintf("%s:%d", a.pk, a.port)
}
