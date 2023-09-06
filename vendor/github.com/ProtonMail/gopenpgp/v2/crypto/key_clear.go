package crypto

import (
	"crypto/dsa" //nolint:staticcheck
	"crypto/rsa"
	"errors"
	"math/big"

	"github.com/ProtonMail/go-crypto/openpgp/ecdh"
	"github.com/ProtonMail/go-crypto/openpgp/ecdsa"
	"github.com/ProtonMail/go-crypto/openpgp/eddsa"
	"github.com/ProtonMail/go-crypto/openpgp/elgamal"
)

func (sk *SessionKey) Clear() (ok bool) {
	clearMem(sk.Key)
	return true
}

func (key *Key) ClearPrivateParams() (ok bool) {
	num := key.clearPrivateWithSubkeys()
	key.entity.PrivateKey = nil

	for k := range key.entity.Subkeys {
		key.entity.Subkeys[k].PrivateKey = nil
	}

	return num > 0
}

func (key *Key) clearPrivateWithSubkeys() (num int) {
	num = 0
	if key.entity.PrivateKey != nil {
		err := clearPrivateKey(key.entity.PrivateKey.PrivateKey)
		if err == nil {
			num++
		}
	}
	for k := range key.entity.Subkeys {
		if key.entity.Subkeys[k].PrivateKey != nil {
			err := clearPrivateKey(key.entity.Subkeys[k].PrivateKey.PrivateKey)
			if err == nil {
				num++
			}
		}
	}
	return num
}

func clearPrivateKey(privateKey interface{}) error {
	switch priv := privateKey.(type) {
	case *rsa.PrivateKey:
		return clearRSAPrivateKey(priv)
	case *dsa.PrivateKey:
		return clearDSAPrivateKey(priv)
	case *elgamal.PrivateKey:
		return clearElGamalPrivateKey(priv)
	case *ecdsa.PrivateKey:
		return clearECDSAPrivateKey(priv)
	case *eddsa.PrivateKey:
		return clearEdDSAPrivateKey(priv)
	case *ecdh.PrivateKey:
		return clearECDHPrivateKey(priv)
	default:
		return errors.New("gopenpgp: unknown private key")
	}
}

func clearBigInt(n *big.Int) {
	w := n.Bits()
	for k := range w {
		w[k] = 0x00
	}
}

func clearMem(w []byte) {
	for k := range w {
		w[k] = 0x00
	}
}

func clearRSAPrivateKey(rsaPriv *rsa.PrivateKey) error {
	clearBigInt(rsaPriv.D)
	for idx := range rsaPriv.Primes {
		clearBigInt(rsaPriv.Primes[idx])
	}
	clearBigInt(rsaPriv.Precomputed.Qinv)
	clearBigInt(rsaPriv.Precomputed.Dp)
	clearBigInt(rsaPriv.Precomputed.Dq)

	for idx := range rsaPriv.Precomputed.CRTValues {
		clearBigInt(rsaPriv.Precomputed.CRTValues[idx].Exp)
		clearBigInt(rsaPriv.Precomputed.CRTValues[idx].Coeff)
		clearBigInt(rsaPriv.Precomputed.CRTValues[idx].R)
	}

	return nil
}

func clearDSAPrivateKey(priv *dsa.PrivateKey) error {
	clearBigInt(priv.X)

	return nil
}

func clearElGamalPrivateKey(priv *elgamal.PrivateKey) error {
	clearBigInt(priv.X)

	return nil
}

func clearECDSAPrivateKey(priv *ecdsa.PrivateKey) error {
	clearBigInt(priv.D)

	return nil
}

func clearEdDSAPrivateKey(priv *eddsa.PrivateKey) error {
	clearMem(priv.D)

	return nil
}

func clearECDHPrivateKey(priv *ecdh.PrivateKey) error {
	clearMem(priv.D)

	return nil
}
