package cryptomator

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
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
	ID                  string `json:"-"`
	Format              uint   `json:"format"`
	ShorteningThreshold int    `json:"shorteningThreshold"`
	CipherCombo         string `json:"cipherCombo"`
}

// newDefaultVaultConfig creates a new VaultConfig with the default settings
func newDefaultVaultConfig() vaultConfig {
	return vaultConfig{
		ID:                  uuid.NewString(),
		Format:              8,
		ShorteningThreshold: 220,
		CipherCombo:         cipherComboSivGcm,
	}
}

// marshal makes a signed JWT from the VaultConfig.
func (c vaultConfig) marshal(masterKey masterKey) ([]byte, error) {
	keyID := keyID("masterkeyfile:" + masterKeyFileName)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, &vaultJwtClaims{vaultConfig: c, RegisteredClaims: jwt.RegisteredClaims{ID: c.ID}})
	token.Header[configKeyIDTag] = string(keyID)
	rawToken, err := token.SignedString(masterKey.jwtKey())
	if err != nil {
		return nil, err
	}
	return []byte(rawToken), nil
}

type vaultJwtClaims struct {
	vaultConfig
	jwt.RegisteredClaims
}

// unmarshalVaultConfig parses the JWT
func unmarshalVaultConfig(tokenBytes []byte, keyFunc func(masterKeyPath string) (*masterKey, error)) (c vaultConfig, err error) {
	var claims vaultJwtClaims
	_, err = jwt.ParseWithClaims(string(tokenBytes), &claims, func(token *jwt.Token) (any, error) {
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
	c = claims.vaultConfig
	c.ID = claims.RegisteredClaims.ID
	return
}

func (f *Fs) loadVault(ctx context.Context, passphrase string) error {
	configData, err := f.readSmallFile(ctx, configFileName, 1024)
	if err == nil {
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
	} else if errors.Is(err, fs.ErrorObjectNotFound) {
		// try to load older config version with no jwt, only master key
		masterKeyData, err := f.readSmallFile(ctx, masterKeyFileName, 1024)
		if err != nil {
			return fmt.Errorf("failed to read master key: %w", err)
		}
		f.masterKey, err = unmarshalMasterKey(bytes.NewReader(masterKeyData), passphrase)
		if err != nil {
			return err
		}
		f.vaultConfig = vaultConfig{
			ID:                  "aaa",
			ShorteningThreshold: newDefaultVaultConfig().ShorteningThreshold,
			Format:              uint(f.masterKey.Version),
			CipherCombo:         cipherComboSivCtrMac,
		}
	} else {
		return fmt.Errorf("failed to read config at %s: %w", configFileName, err)
	}

	switch f.vaultConfig.Format {
	case 8, 7:
	default:
		return fmt.Errorf("unsupported vault format: %d", f.vaultConfig.Format)
	}

	return nil
}

func (f *Fs) createVault(ctx context.Context, passphrase string, vaultConfig vaultConfig) error {
	switch vaultConfig.Format {
	case 8:
		return f.createVaultV8(ctx, passphrase, vaultConfig)
	case 7:
		return f.createVaultV7(ctx, passphrase, vaultConfig)
	default:
		return fmt.Errorf("unsupported vault format: %d", f.vaultConfig.Format)
	}
}

func (f *Fs) createVaultV8(ctx context.Context, passphrase string, vaultConfig vaultConfig) error {
	masterKey, err := newMasterKey()
	if err != nil {
		return fmt.Errorf("failed to create master key: %w", err)
	}
	buf := bytes.Buffer{}
	err = masterKey.marshal(&buf, passphrase)
	if err != nil {
		return fmt.Errorf("failed to encrypt master key: %w", err)
	}
	err = f.writeSmallFile(ctx, masterKeyFileName, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to save master key: %w", err)
	}

	configBytes, err := vaultConfig.marshal(masterKey)
	if err != nil {
		return fmt.Errorf("failed to encrypt vault config: %w", err)
	}
	err = f.writeSmallFile(ctx, configFileName, configBytes)
	if err != nil {
		return fmt.Errorf("failed to save master key: %w", err)
	}

	return nil
}

func (f *Fs) createVaultV7(ctx context.Context, passphrase string, _ vaultConfig) error {
	masterKey, err := newMasterKey()
	masterKey.Version = 7
	if err != nil {
		return fmt.Errorf("failed to create master key: %w", err)
	}
	buf := bytes.Buffer{}
	err = masterKey.marshal(&buf, passphrase)
	if err != nil {
		return fmt.Errorf("failed to encrypt master key: %w", err)
	}
	err = f.writeSmallFile(ctx, masterKeyFileName, buf.Bytes())
	if err != nil {
		return fmt.Errorf("failed to save master key: %w", err)
	}

	return nil
}
