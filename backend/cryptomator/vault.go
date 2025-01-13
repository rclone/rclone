package cryptomator

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

const (
	ConfigKeyIDTag = "kid"
	ConfigFileName = "vault.cryptomator"
)

type keyID string

func (kid keyID) Scheme() string {
	return strings.Split(string(kid), ":")[0]
}

func (kid keyID) URI() string {
	return strings.Split(string(kid), ":")[1]
}

type VaultConfig struct {
	Format              int    `json:"format"`
	ShorteningThreshold int    `json:"shorteningThreshold"`
	Jti                 string `json:"jti"`
	CipherCombo         string `json:"cipherCombo"`

	KeyID    keyID  `json:"-"`
	rawToken string `json:"-"`
}

func NewVaultConfig(encKey, macKey []byte) (c VaultConfig, err error) {
	masterKeyFileName := "masterkey.cryptomator"
	c = VaultConfig{
		Format:              8,
		ShorteningThreshold: 220,
		Jti:                 uuid.NewString(),
		CipherCombo:         CipherComboSivGcm,
		KeyID:               keyID("masterkeyfile:" + masterKeyFileName),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &c)
	token.Header[ConfigKeyIDTag] = string(c.KeyID)

	c.rawToken, err = token.SignedString(append(encKey, macKey...))

	return
}

func (c *VaultConfig) Valid() error {
	if c.Format != 8 {
		return fmt.Errorf("unsupported vault format: %d", c.Format)
	}

	return nil
}

func (c VaultConfig) Marshal(w io.Writer, encKey, macKey []byte) error {
	_, err := w.Write([]byte(c.rawToken))

	return err
}

func (c VaultConfig) Verify(encKey, macKey []byte) error {
	_, err := jwt.Parse(c.rawToken, func(t *jwt.Token) (interface{}, error) {
		return append(encKey, macKey...), nil
	})

	return err
}

func UnmarshalUnverifiedVaultConfig(r io.Reader) (c VaultConfig, err error) {
	tokenBytes, err := io.ReadAll(r)
	if err != nil {
		return
	}

	token, _, err := jwt.NewParser().ParseUnverified(string(tokenBytes), &c)

	if err = token.Claims.Valid(); err != nil {
		return
	}

	c.KeyID = keyID(token.Header[ConfigKeyIDTag].(string))
	c.rawToken = token.Raw

	return
}

func loadVault(ctx context.Context, fs *Fs, password string) error {
	configData, err := fs.readMetadataFile(ctx, "vault.cryptomator", 1024)
	if err != nil {
		return fmt.Errorf("failed to read config at vault.cryptomator: %w", err)
	}
	token, err := jwt.ParseWithClaims(string(configData), &fs.vaultConfig, func(token *jwt.Token) (any, error) {
		kidObj, ok := token.Header["kid"]
		if !ok {
			return nil, fmt.Errorf("no key url in vault.cryptomator jwt")
		}
		kid, ok := kidObj.(string)
		if !ok {
			return nil, fmt.Errorf("key url in vault.cryptomator jwt is not a string")
		}
		masterKeyPath, ok := strings.CutPrefix(kid, "masterkeyfile:")
		if !ok {
			return nil, fmt.Errorf("vault.cryptomator key url does not start with masterkeyfile:")
		}
		masterKeyData, err := fs.readMetadataFile(ctx, masterKeyPath, 1024)
		if err != nil {
			return nil, fmt.Errorf("failed to read master key: %w", err)
		}
		fs.masterKey, err = UnmarshalMasterKey(bytes.NewReader(masterKeyData), password)
		if err != nil {
			return nil, err
		}
		return append(append([]byte{}, fs.masterKey.EncryptKey...), fs.masterKey.MacKey...), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	if err != nil {
		return fmt.Errorf("failed to parse jwt: %w", err)
	}
	if !token.Valid {
		return fmt.Errorf("invalid jwt")
	}
	return nil
}
