package cryptomator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/rclone/rclone/fs"
)

const (
	configKeyIDTag    = "kid"
	configFileName    = "vault.cryptomator"
	masterKeyFileName = "masterkey.cryptomator"
)

type keyID string

func (kid keyID) Scheme() string {
	return strings.Split(string(kid), ":")[0]
}

func (kid keyID) URI() string {
	return strings.Split(string(kid), ":")[1]
}

func (m masterKey) jwtKey() []byte {
	return append(m.EncryptKey, m.MacKey...)
}

// vaultConfig is the configuration for the vault, saved in vault.cryptomator at the root of the vault.
type vaultConfig struct {
	Format              int    `json:"format"`
	ShorteningThreshold int    `json:"shorteningThreshold"`
	Jti                 string `json:"jti"`
	CipherCombo         string `json:"cipherCombo"`
}

// newVaultConfig creates a new VaultConfig with the default settings and signs it.
func newVaultConfig() vaultConfig {
	return vaultConfig{
		Format:              8,
		ShorteningThreshold: 220,
		Jti:                 uuid.NewString(),
		CipherCombo:         cipherComboSivGcm,
	}
}

// Valid tests the validity of the VaultConfig during JWT parsing.
func (c *vaultConfig) Valid() error {
	if c.Format != 8 {
		return fmt.Errorf("unsupported vault format: %d", c.Format)
	}

	return nil
}

// Marshal makes a signed JWT from the VaultConfig.
func (c vaultConfig) Marshal(masterKey masterKey) ([]byte, error) {
	keyID := keyID("masterkeyfile:" + masterKeyFileName)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &c)
	token.Header[configKeyIDTag] = string(keyID)
	rawToken, err := token.SignedString(masterKey.jwtKey())
	if err != nil {
		return nil, err
	}
	return []byte(rawToken), nil
}

// unmarshalVaultConfig parses the JWT without verifying it
func unmarshalVaultConfig(tokenBytes []byte, keyFunc func(masterKeyPath string) (*masterKey, error)) (c vaultConfig, err error) {
	_, err = jwt.ParseWithClaims(string(tokenBytes), &c, func(token *jwt.Token) (any, error) {
		kidObj, ok := token.Header[configKeyIDTag]
		if !ok {
			return nil, fmt.Errorf("no key url in vault.cryptomator jwt")
		}
		kid, ok := kidObj.(string)
		if !ok {
			return nil, fmt.Errorf("key url in vault.cryptomator jwt is not a string")
		}
		keyID := keyID(kid)
		masterKey, err := keyFunc(keyID.URI())
		if err != nil {
			return nil, err
		}
		return masterKey.jwtKey(), nil
	}, jwt.WithValidMethods([]string{"HS256", "HS384", "HS512"}))
	return
}

func (f *Fs) loadOrCreateVault(ctx context.Context, passphrase string) error {
	configData, err := f.readSmallFile(ctx, configFileName, 1024)
	if err != nil {
		if !errors.Is(err, fs.ErrorObjectNotFound) {
			return fmt.Errorf("failed to read config at %s: %w", configFileName, err)
		}
		// Vault does not exist, so create it
		err = f.createVault(ctx, passphrase)
		if err != nil {
			return fmt.Errorf("failed to create new vault: %w", err)
		}
		configData, err = f.readSmallFile(ctx, "vault.cryptomator", 1024)
		if err != nil {
			return fmt.Errorf("failed to read vault config after creating new vault: %w", err)
		}
	}

	f.vaultConfig, err = unmarshalVaultConfig(configData, func(masterKeyPath string) (*masterKey, error) {
		masterKeyData, err := f.readSmallFile(ctx, masterKeyPath, 1024)
		if err != nil {
			return nil, fmt.Errorf("failed to read master key: %w", err)
		}
		f.masterKey, err = unmarshalMasterKey(bytes.NewReader(masterKeyData), passphrase)
		if err != nil {
			return nil, err
		}
		return &f.masterKey, nil
	})
	if err != nil {
		return fmt.Errorf("failed to parse jwt: %w", err)
	}
	return nil
}

func (f *Fs) createVault(ctx context.Context, passphrase string) error {
	masterKey, err := newMasterKey()
	if err != nil {
		return fmt.Errorf("failed to create master key: %w", err)
	}
	buf := bytes.Buffer{}
	err = masterKey.Marshal(&buf, passphrase)
	if err != nil {
		return fmt.Errorf("failed to encrypt master key: %w", err)
	}
	err = f.writeSmallFile(ctx, masterKeyFileName, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to save master key: %w", err)
	}

	vaultConfig := newVaultConfig()
	configBytes, err := vaultConfig.Marshal(masterKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault config: %w", err)
	}
	err = f.writeSmallFile(ctx, configFileName, configBytes)
	if err != nil {
		return fmt.Errorf("failed to save master key: %w", err)
	}

	return nil
}
