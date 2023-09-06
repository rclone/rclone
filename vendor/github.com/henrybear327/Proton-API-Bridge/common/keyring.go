package common

import (
	"context"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

/*
The Proton account keys are organized in the following hierarchy.

An account has some users, each of the user will have one or more user keys.
Each of the user will have some addresses, each of the address will have one or more address keys.

A key is encrypted by a passphrase, and the passphrase is encrypted by another key.

The address keyrings are encrypted with the primary user keyring at the time.

The primary address key is used to create (encrypt) and retrieve (decrypt) data, e.g. shares
*/
func getAccountKRs(ctx context.Context, c *proton.Client, keyPass, saltedKeyPass []byte) (*crypto.KeyRing, map[string]*crypto.KeyRing, []proton.Address, []byte, error) {
	/* Code taken and modified from proton-bridge */

	user, err := c.GetUser(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// log.Printf("user %#v", user)

	addr, err := c.GetAddresses(ctx)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	// log.Printf("addr %#v", addr)

	if saltedKeyPass == nil {
		if keyPass == nil {
			return nil, nil, nil, nil, ErrKeyPassOrSaltedKeyPassMustBeNotNil
		}

		/*
			Notes for -> BUG: Access token does not have sufficient scope
			Only within the first x minutes that the user logs in with username and password, the getSalts route will be available to be called!
		*/
		salts, err := c.GetSalts(ctx)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		// log.Printf("salts %#v", salts)

		saltedKeyPass, err = salts.SaltForKey(keyPass, user.Keys.Primary().ID)
		if err != nil {
			return nil, nil, nil, nil, err
		}
		// log.Printf("saltedKeyPass ok")
	}

	userKR, addrKRs, err := proton.Unlock(user, addr, saltedKeyPass, nil)
	if err != nil {
		return nil, nil, nil, nil, err

	} else if userKR.CountDecryptionEntities() == 0 {
		if err != nil {
			return nil, nil, nil, nil, ErrFailedToUnlockUserKeys
		}
	}

	return userKR, addrKRs, addr, saltedKeyPass, nil
}
