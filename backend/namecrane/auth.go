package namecrane

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// AuthManager manages the authentication token.
type AuthManager struct {
	mu        sync.Mutex
	token     string
	expiresAt time.Time
	client    *http.Client
	apiURL    string
	username  string
	password  string
}

// NewAuthManager initializes the AuthManager.
func NewAuthManager(client *http.Client, apiURL, username, password string) *AuthManager {
	return &AuthManager{
		client:   client,
		apiURL:   apiURL,
		username: username,
		password: password,
	}
}

// Authenticate obtains a new token.
func (am *AuthManager) Authenticate(ctx context.Context) error {
	am.mu.Lock()
	defer am.mu.Unlock()

	// If the token is still valid, skip re-authentication
	if time.Now().Before(am.expiresAt) {
		return nil
	}

	// Construct the API URL for authentication
	url := fmt.Sprintf("%s/api/v1/auth/authenticate-user", am.apiURL)

	// Prepare the request body
	requestBody := map[string]string{
		"username": am.username,
		"password": am.password,
	}
	jsonBody, err := json.Marshal(requestBody)
	if err != nil {
		return fmt.Errorf("failed to marshal authentication body: %w", err)
	}

	// Create the HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to create authentication request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	// Execute the request
	resp, err := am.client.Do(req)
	if err != nil {
		return fmt.Errorf("authentication request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("authentication failed, status: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse the response
	var response struct {
		Token     string `json:"accessToken"`
		ExpiresIn string `json:"accessTokenExpiration"` // Token expiration datetime
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return fmt.Errorf("failed to decode authentication response: %w", err)
	}

	// Store the token and expiration time
	am.token = response.Token
	expiresAt, err := time.Parse(time.RFC3339, response.ExpiresIn)
	if err != nil {
		return fmt.Errorf("failed to parse token expiration time: %w", err)
	}
	am.expiresAt = expiresAt

	return nil
}

// GetToken ensures the token is valid and returns it.
func (am *AuthManager) GetToken(ctx context.Context) (string, error) {
	if err := am.Authenticate(ctx); err != nil {
		return "", err
	}
	return am.token, nil
}
