package common

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"os"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/henrybear327/go-proton-api"
)

type ProtonDriveCredential struct {
	UID           string
	AccessToken   string
	RefreshToken  string
	SaltedKeyPass string
}

func cacheCredentialToFile(config *Config) error {
	if config.CredentialCacheFile != "" {
		str, err := json.Marshal(config.ReusableCredential)
		if err != nil {
			return err
		}

		file, err := os.Create(config.CredentialCacheFile)
		if err != nil {
			return err
		}
		defer file.Close()
		_, err = file.WriteString(string(str))
		if err != nil {
			return err
		}
	}

	return nil
}

/*
Log in methods
- username and password to log in
- UID and refresh token

Keyring decryption
The password will be salted, and then used to decrypt the keyring. The salted password needs to be and can be cached, so the keyring can be re-decrypted when needed
*/
func Login(ctx context.Context, config *Config, authHandler proton.AuthHandler, deAuthHandler proton.Handler) (*proton.Manager, *proton.Client, *ProtonDriveCredential, *crypto.KeyRing, map[string]*crypto.KeyRing, []proton.Address, error) {
	var c *proton.Client
	var auth proton.Auth
	var userKR *crypto.KeyRing
	var addrKRs map[string]*crypto.KeyRing
	var addr []proton.Address

	// get manager
	m := getProtonManager(config.AppVersion, config.UserAgent)

	if config.UseReusableLogin {
		c = m.NewClient(config.ReusableCredential.UID, config.ReusableCredential.AccessToken, config.ReusableCredential.RefreshToken)
		c.AddAuthHandler(authHandler)
		c.AddDeauthHandler(deAuthHandler)

		err := cacheCredentialToFile(config)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}

		SaltedKeyPassByteArr, err := base64.StdEncoding.DecodeString(config.ReusableCredential.SaltedKeyPass)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}
		userKR, addrKRs, addr, _, err = getAccountKRs(ctx, c, nil, SaltedKeyPassByteArr)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}

		return m, c, nil, userKR, addrKRs, addr, nil
	} else {
		username := config.FirstLoginCredential.Username
		password := config.FirstLoginCredential.Password
		if username == "" || password == "" {
			return nil, nil, nil, nil, nil, nil, ErrUsernameAndPasswordRequired
		}

		// perform login
		var err error
		c, auth, err = m.NewClientWithLogin(ctx, username, []byte(password))
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}
		c.AddAuthHandler(authHandler)
		c.AddDeauthHandler(deAuthHandler)

		if auth.TwoFA.Enabled&proton.HasTOTP != 0 {
			if config.FirstLoginCredential.TwoFA != "" {
				err := c.Auth2FA(ctx, proton.Auth2FAReq{
					TwoFactorCode: config.FirstLoginCredential.TwoFA,
				})
				if err != nil {
					return nil, nil, nil, nil, nil, nil, err
				}
			} else {
				return nil, nil, nil, nil, nil, nil, Err2FACodeRequired
			}
		}

		// decrypt keyring
		var saltedKeyPassByteArr []byte
		userKR, addrKRs, addr, saltedKeyPassByteArr, err = getAccountKRs(ctx, c, []byte(password), nil)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}

		saltedKeyPass := base64.StdEncoding.EncodeToString(saltedKeyPassByteArr)
		config.ReusableCredential.UID = auth.UID
		config.ReusableCredential.AccessToken = auth.AccessToken
		config.ReusableCredential.RefreshToken = auth.RefreshToken
		config.ReusableCredential.SaltedKeyPass = saltedKeyPass

		err = cacheCredentialToFile(config)
		if err != nil {
			return nil, nil, nil, nil, nil, nil, err
		}

		return m, c, &ProtonDriveCredential{
			UID:           auth.UID,
			AccessToken:   auth.AccessToken,
			RefreshToken:  auth.RefreshToken,
			SaltedKeyPass: saltedKeyPass,
		}, userKR, addrKRs, addr, nil
	}
}

func Logout(ctx context.Context, config *Config, m *proton.Manager, c *proton.Client, userKR *crypto.KeyRing, addrKRs map[string]*crypto.KeyRing) error {
	defer m.Close()
	defer c.Close()

	if config.CredentialCacheFile == "" {
		log.Println("Logging out user")

		// log out
		err := c.AuthDelete(ctx)
		if err != nil {
			return err
		}

		// clear keyrings
		userKR.ClearPrivateParams()

		for i := range addrKRs {
			addrKRs[i].ClearPrivateParams()
		}
	}

	return nil
}
