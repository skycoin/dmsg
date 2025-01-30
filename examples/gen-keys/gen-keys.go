// example keypair generation
package main

import (
	"fmt"

	"github.com/skycoin/skywire/pkg/skywire-utilities/pkg/cipher"
)

func main() {
	pk, sk := cipher.GenerateKeyPair()
	fmt.Printf("%s\n%s\n", pk, sk)
}
