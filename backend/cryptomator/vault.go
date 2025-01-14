package cryptomator

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
)

const (
	configKeyIDTag = "kid"
	configFileName = "vault.cryptomator"
)

type keyID string

func (kid keyID) Scheme() string {
	return strings.Split(string(kid), ":")[0]
}

func (kid keyID) URI() string {
	return strings.Split(string(kid), ":")[1]
}

func (m MasterKey) jwtKey() []byte {
	return append(m.EncryptKey, m.MacKey...)
}

// VaultConfig is the configuration for the vault, saved in vault.cryptomator at the root of the vault.
type VaultConfig struct {
	Format              int    `json:"format"`
	ShorteningThreshold int    `json:"shorteningThreshold"`
	Jti                 string `json:"jti"`
	CipherCombo         string `json:"cipherCombo"`
}

// NewVaultConfig creates a new VaultConfig with the default settings and signs it.
func NewVaultConfig() VaultConfig {
	return VaultConfig{
		Format:              8,
		ShorteningThreshold: 220,
		Jti:                 uuid.NewString(),
		CipherCombo:         CipherComboSivGcm,
	}
}

// Valid tests the validity of the VaultConfig during JWT parsing.
func (c *VaultConfig) Valid() error {
	if c.Format != 8 {
		return fmt.Errorf("unsupported vault format: %d", c.Format)
	}

	return nil
}

// Marshal makes a signed JWT from the VaultConfig.
func (c VaultConfig) Marshal(masterKey MasterKey) ([]byte, error) {
	masterKeyFileName := "masterkey.cryptomator"
	keyID := keyID("masterkeyfile:" + masterKeyFileName)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &c)
	token.Header[configKeyIDTag] = string(keyID)
	rawToken, err := token.SignedString(masterKey.jwtKey())
	if err != nil {
		return nil, err
	}
	return []byte(rawToken), nil
}

// UnmarshalVaultConfig parses the JWT without verifying it
func UnmarshalVaultConfig(tokenBytes []byte, keyFunc func(token *jwt.Token) (*MasterKey, error)) (c VaultConfig, err error) {
	_, err = jwt.ParseWithClaims(string(tokenBytes), &c, func(token *jwt.Token) (any, error) {
		masterKey, err := keyFunc(token)
		if err != nil {
			return nil, err
		}
		return masterKey.jwtKey(), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	return
}

func loadVault(ctx context.Context, fs *Fs, passphrase string) error {
	configData, err := fs.readSmallFile(ctx, "vault.cryptomator", 1024)
	if err != nil {
		return fmt.Errorf("failed to read config at vault.cryptomator: %w", err)
	}
	fs.vaultConfig, err = UnmarshalVaultConfig(configData, func(token *jwt.Token) (*MasterKey, error) {
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
			return nil, fmt.Errorf("vault.cryptomator key url does not start with \"masterkeyfile:\"")
		}
		masterKeyData, err := fs.readSmallFile(ctx, masterKeyPath, 1024)
		if err != nil {
			return nil, fmt.Errorf("failed to read master key: %w", err)
		}
		fs.masterKey, err = UnmarshalMasterKey(bytes.NewReader(masterKeyData), passphrase)
		if err != nil {
			return nil, err
		}
		return &fs.masterKey, nil
	})
	if err != nil {
		return fmt.Errorf("failed to parse jwt: %w", err)
	}
	return nil
}
