package backend

import (
	_ "embed"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

var modulus string

func init() {
	arm, err := crypto.NewClearTextMessage(asc, sig).GetArmored()
	if err != nil {
		panic(err)
	}

	modulus = arm
}

//go:embed modulus.asc
var asc []byte

//go:embed modulus.sig
var sig []byte
