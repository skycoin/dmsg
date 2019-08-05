package dmsg

import (
	"fmt"

	"github.com/skycoin/dmsg/cipher"
)

type Addr struct {
	pk   cipher.PubKey
	port *uint16
}

func (Addr) Network() string {
	return Type
}

func (a Addr) String() string {
	if a.port == nil {
		return a.pk.String()
	}
	return fmt.Sprintf("%s:%d", a.pk, a.port)
}
