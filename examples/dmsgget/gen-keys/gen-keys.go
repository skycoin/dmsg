package main

import (
	"fmt"

	"github.com/skycoin/dmsg/cipher"
)

func main() {
	pk, sk := cipher.GenerateKeyPair()
	fmt.Println("PK:", pk.String())
	fmt.Println("SK:", sk.String())
}
