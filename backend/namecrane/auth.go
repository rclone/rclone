package namecrane

import (
	"context"
	"errors"
	"fmt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"net/http"
	"sync"
	"time"
)

var (
	ErrNoToken             = errors.New("could not find access token")
	ErrExpiredRefreshToken = errors.New("refresh token expired")
)

const (
	accessTokenKey        = "access_token"
	accessTokenExpireKey  = "access_token_expires"
	refreshTokenKey       = "refresh_token"
	refreshTokenExpireKey = "refresh_token_expires"
)

// AuthManager manages the authentication token.
type AuthManager struct {
	mu           sync.Mutex
	cm           configmap.Mapper
	expiresAt    time.Time
	client       *http.Client
	apiURL       string
	lastResponse *authResponse
}

// NewAuthManager initializes the AuthManager.
func NewAuthManager(client *http.Client, cm configmap.Mapper, apiURL string) *AuthManager {
	return &AuthManager{
		client: client,
		cm:     cm,
		apiURL: apiURL,
	}
}

type authRequest struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	TwoFactorCode string `json:"twoFactorCode"`
}

type authResponse struct {
	Username               string    `json:"username"`
	Token                  string    `json:"accessToken"`
	TokenExpiration        time.Time `json:"accessTokenExpiration"` // Token expiration datetime
	RefreshToken           string    `json:"refreshToken"`
	RefreshTokenExpiration time.Time `json:"refreshTokenExpiration"`
}

func (am *AuthManager) fillFromConfigMapper() error {
	fs.Debugf(am, "Filling last response value from config mapper")

	var response authResponse
	var ok bool
	var err error

	if response.Token, ok = am.cm.Get(accessTokenKey); !ok {
		fs.Debugf(am, "Token not found in config mapper")
		return nil
	} else {
		response.Token, err = obscure.Reveal(response.Token)

		if err != nil {
			return err
		}
	}

	if tokenExpiration, ok := am.cm.Get(accessTokenExpireKey); ok {
		response.TokenExpiration, err = time.Parse(time.RFC3339, tokenExpiration)

		if err != nil {
			return err
		}
	} else {
		fs.Debugf(am, "Token expiration not found in config mapper")
		return nil
	}

	if response.RefreshToken, ok = am.cm.Get(refreshTokenKey); !ok {
		fs.Debugf(am, "Refresh token not found in config mapper")
		return nil
	} else {
		response.RefreshToken, err = obscure.Reveal(response.RefreshToken)

		if err != nil {
			return err
		}
	}

	if refreshTokenExpiration, ok := am.cm.Get(refreshTokenExpireKey); ok {
		var err error
		response.RefreshTokenExpiration, err = time.Parse(time.RFC3339, refreshTokenExpiration)

		if err != nil {
			return err
		}
	} else {
		fs.Debugf(am, "Refresh token expiration not found in config mapper")
		return nil
	}

	fs.Debugf(am, "All information found and filled")

	am.lastResponse = &response

	return nil
}

// Authenticate obtains a new token.
func (am *AuthManager) Authenticate(ctx context.Context, username, password, twoFactorCode string) error {
	fs.Debugf(am, "Trying to authenticate user")

	am.mu.Lock()
	defer am.mu.Unlock()

	// Construct the API URL for authentication
	url := fmt.Sprintf("%s/api/v1/auth/authenticate-user", am.apiURL)

	res, err := doHttpRequest(ctx, am.client, http.MethodPost, url, authRequest{
		Username:      username,
		Password:      password,
		TwoFactorCode: twoFactorCode,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", res.StatusCode)
	}

	// Parse the response
	var response authResponse

	if err := res.Decode(&response); err != nil {
		return fmt.Errorf("failed to decode authenteication response: %w", err)
	}

	// Store the token and expiration time
	am.lastResponse = &response

	am.updateConfigMapper(response)

	return nil
}

type refreshRequest struct {
	Token string `json:"token"`
}

func (am *AuthManager) RefreshToken(ctx context.Context) error {
	fs.Debugf(am, "Trying to refresh token")

	am.mu.Lock()
	defer am.mu.Unlock()

	url := fmt.Sprintf("%s/api/v1/auth/refresh-token", am.apiURL)

	res, err := doHttpRequest(ctx, am.client, http.MethodPost, url, refreshRequest{
		Token: am.lastResponse.RefreshToken,
	})

	if err != nil {
		return err
	}

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code %d", res.StatusCode)
	}

	var response authResponse

	if err := res.Decode(&response); err != nil {
		return fmt.Errorf("failed to decode refresh response: %w", err)
	}

	am.lastResponse = &response

	am.updateConfigMapper(response)

	return nil
}

// updateConfigMapper stores data into the config map
func (am *AuthManager) updateConfigMapper(response authResponse) {
	fs.Debugf(am, "Updating config mapper values")

	am.cm.Set(accessTokenKey, obscure.MustObscure(response.Token))
	am.cm.Set(accessTokenExpireKey, response.TokenExpiration.Format(time.RFC3339))
	am.cm.Set(refreshTokenKey, obscure.MustObscure(response.RefreshToken))
	am.cm.Set(refreshTokenExpireKey, response.RefreshTokenExpiration.Format(time.RFC3339))
}

// GetToken ensures the token is valid and returns it.
func (am *AuthManager) GetToken(ctx context.Context) (string, error) {
	if am.lastResponse == nil || am.lastResponse.Token == "" {
		fs.Debugf(am, "No token set in AuthManager")
		return "", ErrNoToken
	}

	// Handle if we can't use our refresh token
	if am.lastResponse.RefreshTokenExpiration.Before(time.Now()) {
		fs.Debugf(am, "Refresh token expired")
		return "", ErrExpiredRefreshToken
	}

	// Give us a 5 minute grace period to prevent race conditions/issues
	if am.lastResponse.TokenExpiration.Before(time.Now().Add(5 * time.Minute)) {
		fs.Debugf(am, "Access token expires soon, need to refresh")

		// Refresh token
		if err := am.RefreshToken(ctx); err != nil {
			return "", fmt.Errorf("failed to refresh token: %w", err)
		}
	}

	fs.Debug(am, "Using existing token")

	return am.lastResponse.Token, nil
}

func (am *AuthManager) String() string {
	return "Namecrane Auth Manager"
}
