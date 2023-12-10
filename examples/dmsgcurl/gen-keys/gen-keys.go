package main

import (
	"fmt"

	"github.com/skycoin/skywire-utilities/pkg/cipher"
)

func main() {
	pk, sk := cipher.GenerateKeyPair()
	fmt.Println("PK:", pk.String())
	fmt.Println("SK:", sk.String())
}
