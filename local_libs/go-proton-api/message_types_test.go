package proton

import (
	"os"
	"testing"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/stretchr/testify/require"
)

func TestDecrypt(t *testing.T) {
	body, err := os.ReadFile("testdata/body.pgp")
	require.NoError(t, err)

	pubKR := loadKeyRing(t, "testdata/pub.asc", nil)
	prvKR := loadKeyRing(t, "testdata/prv.asc", []byte("password"))

	msg := Message{Body: string(body)}

	sigs, err := ExtractSignatures(prvKR, msg.Body)
	require.NoError(t, err)

	enc, err := crypto.NewPGPMessageFromArmored(msg.Body)
	require.NoError(t, err)

	dec, err := prvKR.Decrypt(enc, nil, crypto.GetUnixTime())
	require.NoError(t, err)
	require.NoError(t, pubKR.VerifyDetached(dec, sigs[0].Data, crypto.GetUnixTime()))
}

func loadKeyRing(t *testing.T, file string, pass []byte) *crypto.KeyRing {
	f, err := os.Open(file)
	require.NoError(t, err)

	defer f.Close()

	key, err := crypto.NewKeyFromArmoredReader(f)
	require.NoError(t, err)

	if pass != nil {
		key, err = key.Unlock(pass)
		require.NoError(t, err)
	}

	kr, err := crypto.NewKeyRing(key)
	require.NoError(t, err)

	return kr
}
