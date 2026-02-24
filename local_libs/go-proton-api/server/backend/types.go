package backend

import (
	"encoding/base64"
	"math/big"
	"time"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
)

type ID uint64

func (v ID) String() string {
	return base64.URLEncoding.EncodeToString(v.Bytes())
}

func (v ID) Bytes() []byte {
	if v == 0 {
		return []byte{0}
	}

	return new(big.Int).SetUint64(uint64(v)).Bytes()
}

func (v *ID) FromString(s string) error {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return err
	}

	*v = ID(new(big.Int).SetBytes(b).Uint64())

	return nil
}

type auth struct {
	acc string
	ref string

	creation time.Time
}

func newAuth(authLife time.Duration) auth {
	return auth{
		acc: uuid.NewString(),
		ref: uuid.NewString(),

		creation: time.Now(),
	}
}

func (auth *auth) toAuth(userID, authUID string, proof []byte) proton.Auth {
	return proton.Auth{
		UserID: userID,

		UID:          authUID,
		AccessToken:  auth.acc,
		RefreshToken: auth.ref,
		ServerProof:  base64.StdEncoding.EncodeToString(proof),

		PasswordMode: proton.OnePasswordMode,
	}
}

func (auth *auth) toAuthSession(authUID string) proton.AuthSession {
	return proton.AuthSession{
		UID:        authUID,
		CreateTime: auth.creation.Unix(),
		Revocable:  true,
	}
}

type key struct {
	keyID string
	key   string
	tok   string
	sig   string
}

func (key key) unlock(passphrase []byte) (*crypto.KeyRing, error) {
	lockedKey, err := crypto.NewKeyFromArmored(key.key)
	if err != nil {
		return nil, err
	}

	unlockedKey, err := lockedKey.Unlock(passphrase)
	if err != nil {
		return nil, err
	}

	return crypto.NewKeyRing(unlockedKey)
}

func (key key) getPubKey() (*crypto.Key, error) {
	privKey, err := crypto.NewKeyFromArmored(key.key)
	if err != nil {
		return nil, err
	}

	pubKeyBin, err := privKey.GetPublicKey()
	if err != nil {
		return nil, err
	}

	return crypto.NewKey(pubKeyBin)
}
