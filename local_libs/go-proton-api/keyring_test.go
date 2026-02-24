package proton

import (
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/ProtonMail/gopenpgp/v2/helper"
	"github.com/stretchr/testify/require"
)

func TestKeyring_Unlock(t *testing.T) {
	r := require.New(t)

	newKey := func(id, passphrase string) Key {
		arm, err := helper.GenerateKey(id, id+"@email.com", []byte(passphrase), "rsa", 2048)
		r.NoError(err)

		privKey, err := crypto.NewKeyFromArmored(arm)
		r.NoError(err)

		serial, err := privKey.Serialize()
		r.NoError(err)

		return Key{
			ID:         id,
			PrivateKey: serial,
			Active:     true,
		}
	}

	keys := Keys{
		newKey("1", "good_phrase"),
		newKey("2", "good_phrase"),
		newKey("3", "bad_phrase"),
	}

	_, err := keys.Unlock([]byte("ugly_phrase"), nil)
	r.Error(err)

	kr, err := keys.Unlock([]byte("bad_phrase"), nil)
	r.NoError(err)
	r.Equal(1, kr.CountEntities())

	kr, err = keys.Unlock([]byte("good_phrase"), nil)
	r.NoError(err)
	r.Equal(2, kr.CountEntities())
}
