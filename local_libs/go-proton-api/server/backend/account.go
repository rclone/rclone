package backend

import (
	"sync"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/xslices"
	"github.com/google/uuid"
	"github.com/rclone/go-proton-api"
)

type account struct {
	userID       string
	username     string
	addresses    map[string]*address
	mailSettings *mailSettings
	userSettings proton.UserSettings

	auth     map[string]auth
	authLock sync.RWMutex

	keys     []key
	salt     []byte
	verifier []byte

	labelIDs   []string
	messageIDs []string
	updateIDs  []ID
}

func newAccount(userID, username string, armKey string, salt, verifier []byte) *account {
	return &account{
		userID:       userID,
		username:     username,
		addresses:    make(map[string]*address),
		mailSettings: newMailSettings(username),
		userSettings: newUserSettings(),

		auth:     make(map[string]auth),
		keys:     []key{{keyID: uuid.NewString(), key: armKey}},
		salt:     salt,
		verifier: verifier,
	}
}

func (acc *account) toUser() proton.User {
	return proton.User{
		ID:          acc.userID,
		Name:        acc.username,
		DisplayName: acc.username,
		Email:       acc.primary().email,
		Keys: xslices.Map(acc.keys, func(key key) proton.Key {
			privKey, err := crypto.NewKeyFromArmored(key.key)
			if err != nil {
				panic(err)
			}

			rawKey, err := privKey.Serialize()
			if err != nil {
				panic(err)
			}

			return proton.Key{
				ID:         key.keyID,
				PrivateKey: rawKey,
				Primary:    key == acc.keys[0],
				Active:     true,
			}
		}),
	}
}

func (acc *account) primary() *address {
	for _, addr := range acc.addresses {
		if addr.order == 1 {
			return addr
		}
	}

	panic("no primary address")
}

func (acc *account) getAddr(email string) (*address, bool) {
	for _, addr := range acc.addresses {
		if addr.email == email {
			return addr, true
		}
	}

	return nil, false
}
