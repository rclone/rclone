package proton

import (
	"encoding/json"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

type PublicKey struct {
	Flags     KeyState
	PublicKey string
}

type PublicKeys []PublicKey

func (keys PublicKeys) GetKeyRing() (*crypto.KeyRing, error) {
	kr, err := crypto.NewKeyRing(nil)
	if err != nil {
		return nil, err
	}

	for _, key := range keys {
		pubKey, err := crypto.NewKeyFromArmored(key.PublicKey)
		if err != nil {
			return nil, err
		}

		if err := kr.AddKey(pubKey); err != nil {
			return nil, err
		}
	}

	return kr, nil
}

type KeyList struct {
	Data      string
	Signature string
}

func NewKeyList(signer *crypto.KeyRing, entries []KeyListEntry) (KeyList, error) {
	data, err := json.Marshal(entries)
	if err != nil {
		return KeyList{}, err
	}

	sig, err := signer.SignDetached(crypto.NewPlainMessage(data))
	if err != nil {
		return KeyList{}, err
	}

	arm, err := sig.GetArmored()
	if err != nil {
		return KeyList{}, err
	}

	return KeyList{
		Data:      string(data),
		Signature: arm,
	}, nil
}

type KeyListEntry struct {
	Fingerprint        string
	SHA256Fingerprints []string
	Flags              KeyState
	Primary            Bool
}

type KeyState int

const (
	KeyStateTrusted KeyState = 1 << iota // 2^0 = 1 means the key is not compromised (i.e. if we can trust signatures coming from it)
	KeyStateActive                       // 2^1 = 2 means the key is still in use (i.e. not obsolete, we can encrypt messages to it)
)

type CreateAddressKeyReq struct {
	AddressID     string
	PrivateKey    string
	Primary       Bool
	SignedKeyList KeyList

	// The following are only used in "migrated accounts"
	Token     string `json:",omitempty"`
	Signature string `json:",omitempty"`
}

type MakeAddressKeyPrimaryReq struct {
	SignedKeyList KeyList
}
