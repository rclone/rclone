package proton

import (
	"encoding/base64"
	"fmt"

	"github.com/ProtonMail/go-srp"
	"github.com/bradenaw/juniper/xslices"
)

type Salt struct {
	ID, KeySalt string
}

type Salts []Salt

func (salts Salts) SaltForKey(keyPass []byte, keyID string) ([]byte, error) {
	idx := xslices.IndexFunc(salts, func(salt Salt) bool {
		return salt.ID == keyID
	})

	if idx < 0 {
		return nil, fmt.Errorf("no salt found for key %s", keyID)
	}

	keySalt, err := base64.StdEncoding.DecodeString(salts[idx].KeySalt)
	if err != nil {
		return nil, err
	}

	saltedKeyPass, err := srp.MailboxPassword(keyPass, keySalt)
	if err != nil {
		return nil, nil
	}

	return saltedKeyPass[len(saltedKeyPass)-31:], nil
}
