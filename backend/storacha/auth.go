package storacha

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"encoding/base64"

	"github.com/storacha/go-ucanto/core/delegation"
	"github.com/storacha/go-ucanto/core/result"
	"github.com/storacha/go-ucanto/did"
	"github.com/storacha/go-ucanto/principal"
	"github.com/storacha/go-ucanto/principal/ed25519/signer"
	"github.com/storacha/guppy/pkg/client"
	guppyDelegation "github.com/storacha/guppy/pkg/delegation"
)

var CachedClients = make(map[string]*client.Client)

func EmailAuth(email string) (*client.Client, error) {
	if _, ok := CachedClients[email]; ok {
		return CachedClients[email], nil
	}
	CachedClients[email], _ = emailAuth(email)
	return CachedClients[email], nil
}

func emailAuth(email string) (*client.Client, error) {
	ctx := context.Background()

	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid email: %s", email)
	}
	emailUser, emailDomain := parts[0], parts[1]

	account, err := did.Parse("did:mailto:" + emailDomain + ":" + emailUser)
	if err != nil {
		return nil, err
	}

	c, _ := client.NewClient()

	authOk, err := c.RequestAccess(ctx, account.String())
	if err != nil {
		return nil, err
	}

	resultChan := c.PollClaim(ctx, authOk)
	fmt.Println("Please click the link in your email to authenticate...")
	proofs, err := result.Unwrap(<-resultChan)
	if err != nil {
		return nil, err
	}

	if err := c.AddProofs(proofs...); err != nil {
		return nil, fmt.Errorf("failed to add proofs: %w", err)
	}

	return c, nil
}

// AuthConfig holds the private key authentication configuration
type AuthConfig struct {
	PrivateKeyPath string
	ProofPath      string
	SpaceDID       string
}

// PrivateKeyAuth creates an authenticated client using private key + proofs
func PrivateKeyAuth(config *AuthConfig) (*client.Client, error) {
	cacheKey := fmt.Sprintf("pk:%s:%s:%s", config.PrivateKeyPath, config.ProofPath, config.SpaceDID)

	if cl, ok := CachedClients[cacheKey]; ok {
		return cl, nil
	}

	issuer, err := loadPrivateKey(config.PrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load private key: %w", err)
	}

	proofs, err := loadProofs(config.ProofPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load proofs: %w", err)
	}

	spaceDID, err := did.Parse(config.SpaceDID)
	if err != nil {
		return nil, fmt.Errorf("invalid space DID: %w", err)
	}

	c, err := client.NewClient(client.WithPrincipal(issuer))
	if err != nil {
		return nil, fmt.Errorf("failed to create client with principal: %w", err)
	}

	// Add proofs to the client
	if err := c.AddProofs(proofs...); err != nil {
		return nil, fmt.Errorf("failed to add proofs to client: %w", err)
	}

	CachedClients[cacheKey] = c

	// issuer implements principal.Signer so we can call DID() on it
	fmt.Printf("✓ Authenticated with private key DID: %s\n", issuer.DID().String())
	fmt.Printf("✓ Using space: %s\n", spaceDID.String())

	return c, nil
}

func loadPrivateKey(privateKeyPath string) (principal.Signer, error) {

	if privateKeyPath == "" {
		return nil, fmt.Errorf("private key path is empty")
	}
	if privateKeyPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		privateKeyPath = filepath.Join(homeDir, privateKeyPath[1:])
	}

	keyData, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key file '%s': %w", privateKeyPath, err)
	}

	keyString := strings.TrimSpace(string(keyData))

	// decoding base64 private key (getting issue here)
	keybytes, err := base64.StdEncoding.DecodeString(keyString)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %w", err)
	}

	issuer, err := signer.FromRaw(keybytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	fmt.Printf("Successfully parsed private key\n")
	return issuer, nil
}

func loadProofs(proofPath string) ([]delegation.Delegation, error) {
	// Expand home directory if needed
	if proofPath == "" {
		return nil, fmt.Errorf("proof path is empty")
	}
	if proofPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get home directory: %w", err)
		}
		proofPath = filepath.Join(homeDir, proofPath[1:])
	}

	prfbytes, err := os.ReadFile(proofPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read proof file '%s': %w", proofPath, err)
	}

	proof, err := guppyDelegation.ExtractProof(prfbytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse proof: %w", err)
	}

	return []delegation.Delegation{proof}, nil
}

// LoadAuthConfigFromFlags creates auth config from command line parameters
func LoadAuthConfigFromFlags(privateKeyPath, proofPath, spaceDID string) *AuthConfig {
	return &AuthConfig{
		PrivateKeyPath: privateKeyPath,
		ProofPath:      proofPath,
		SpaceDID:       spaceDID,
	}
}

// LoadAuthConfigFromEnv creates auth config from environment variables
func LoadAuthConfigFromEnv() (*AuthConfig, error) {
	privateKeyPath := os.Getenv("STORACHA_PRIVATE_KEY_PATH")
	proofPath := os.Getenv("STORACHA_PROOF_PATH")
	spaceDID := os.Getenv("STORACHA_SPACE_DID")

	if privateKeyPath == "" {
		return nil, fmt.Errorf("STORACHA_PRIVATE_KEY_PATH environment variable is required")
	}
	if proofPath == "" {
		return nil, fmt.Errorf("STORACHA_PROOF_PATH environment variable is required")
	}
	if spaceDID == "" {
		return nil, fmt.Errorf("STORACHA_SPACE_DID environment variable is required")
	}

	return &AuthConfig{
		PrivateKeyPath: privateKeyPath,
		ProofPath:      proofPath,
		SpaceDID:       spaceDID,
	}, nil
}

// ValidateAuthConfig validates that all required files exist and are readable
func ValidateAuthConfig(config *AuthConfig) error {
	privateKeyPath := config.PrivateKeyPath
	proofPath := config.ProofPath

	if privateKeyPath == "" {
		return fmt.Errorf("private key path is empty")
	}
	if proofPath == "" {
		return fmt.Errorf("proof path is empty")
	}

	if privateKeyPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil || homeDir == "" {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		privateKeyPath = filepath.Join(homeDir, privateKeyPath[1:])
	}

	if proofPath[0] == '~' {
		homeDir, err := os.UserHomeDir()
		if err != nil || homeDir == "" {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		proofPath = filepath.Join(homeDir, proofPath[1:])
	}

	// Validate private key file
	if _, err := os.Stat(privateKeyPath); os.IsNotExist(err) {
		return fmt.Errorf("private key file does not exist: %s", privateKeyPath)
	}

	// Validate proof file
	if _, err := os.Stat(proofPath); os.IsNotExist(err) {
		return fmt.Errorf("proof file does not exist: %s", proofPath)
	}

	// Validate space DID format
	if _, err := did.Parse(config.SpaceDID); err != nil {
		return fmt.Errorf("invalid space DID format: %w", err)
	}

	return nil
}