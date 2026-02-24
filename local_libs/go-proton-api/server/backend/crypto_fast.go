package backend

import "github.com/ProtonMail/gopenpgp/v2/crypto"

var preCompKey *crypto.Key

func init() {
	key, err := crypto.GenerateKey("name", "email", "rsa", 1024)
	if err != nil {
		panic(err)
	}

	preCompKey = key
}

// FastGenerateKey is a fast version of GenerateKey that uses a pre-computed key.
// This is useful for testing but is incredibly insecure.
func FastGenerateKey(_, _ string, passphrase []byte, _ string, _ int) (string, error) {
	encKey, err := preCompKey.Lock(passphrase)
	if err != nil {
		return "", err
	}

	return encKey.Armor()
}
