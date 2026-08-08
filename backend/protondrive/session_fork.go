package protondrive

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/rclone/rclone/fs"
)

const (
	protonWebAuthClientID     = "external-drive"
	protonWebAuthPollInterval = 5 * time.Second
	protonWebAuthInitialDelay = 5 * time.Second
	protonWebAuthMaxPollTime  = 10 * time.Minute
)

var (
	protonDriveAPIBaseURL = "https://drive-api.proton.me"
	protonAccountURL      = "https://account.proton.me"
	protonForkAAD         = []byte("fork")
)

type protonForkInitResponse struct {
	Code     int    `json:"Code"`
	Selector string `json:"Selector"`
	UserCode string `json:"UserCode"`
}

type protonForkStatusResponse struct {
	Code         int    `json:"Code"`
	Payload      string `json:"Payload"`
	UID          string `json:"UID"`
	AccessToken  string `json:"AccessToken"`
	RefreshToken string `json:"RefreshToken"`
}

type protonForkPayload struct {
	KeyPassword string `json:"keyPassword"`
}

type protonWebCredential struct {
	UID           string
	AccessToken   string
	RefreshToken  string
	SaltedKeyPass string
}

func protonDriveAuthViaWeb(ctx context.Context, f *Fs, appVersion, userAgent string, transport http.RoundTripper) (*protonWebCredential, error) {
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}
	if client.Transport == nil {
		client.Transport = http.DefaultTransport
	}

	initResponse, err := protonSessionForkInit(ctx, client, appVersion, userAgent)
	if err != nil {
		return nil, err
	}
	if initResponse.Selector == "" || initResponse.UserCode == "" {
		return nil, errors.New("proton session fork response did not include selector and user code")
	}

	encryptionKey, signInURL, err := protonGenerateSignInURL(initResponse.UserCode)
	if err != nil {
		return nil, err
	}

	fs.Logf(f, "Open this Proton login URL in your browser:")
	fs.Logf(f, "%s", signInURL)

	if err := sleepContext(ctx, protonWebAuthInitialDelay); err != nil {
		return nil, err
	}

	deadline := time.Now().Add(protonWebAuthMaxPollTime)
	for {
		if time.Now().After(deadline) {
			return nil, errors.New("proton browser authentication timed out")
		}

		statusResponse, ready, err := protonSessionForkStatus(ctx, client, appVersion, userAgent, initResponse.Selector)
		if err != nil {
			return nil, err
		}
		if !ready {
			fs.Debugf(f, "Proton browser authentication is not ready yet")
			if err := sleepContext(ctx, protonWebAuthPollInterval); err != nil {
				return nil, err
			}
			continue
		}

		keyPassword, err := protonParseForkKeyPassword(encryptionKey, statusResponse.Payload)
		if err != nil {
			return nil, err
		}
		if statusResponse.UID == "" || statusResponse.AccessToken == "" || statusResponse.RefreshToken == "" {
			return nil, errors.New("proton browser authentication response did not include complete session tokens")
		}

		return &protonWebCredential{
			UID:           statusResponse.UID,
			AccessToken:   statusResponse.AccessToken,
			RefreshToken:  statusResponse.RefreshToken,
			SaltedKeyPass: normalizeSaltedKeyPass(keyPassword),
		}, nil
	}
}

func protonSessionForkInit(ctx context.Context, client *http.Client, appVersion, userAgent string) (*protonForkInitResponse, error) {
	var response protonForkInitResponse
	if err := protonJSONRequest(ctx, client, http.MethodGet, protonDriveAPIBaseURL+"/auth/v4/sessions/forks", appVersion, userAgent, nil, &response); err != nil {
		return nil, err
	}
	return &response, nil
}

func protonSessionForkStatus(ctx context.Context, client *http.Client, appVersion, userAgent, selector string) (*protonForkStatusResponse, bool, error) {
	var response protonForkStatusResponse
	statusCode, err := protonJSONRequestStatus(ctx, client, http.MethodGet, protonDriveAPIBaseURL+"/auth/v4/sessions/forks/"+url.PathEscape(selector), appVersion, userAgent, nil, &response)
	if statusCode == http.StatusUnprocessableEntity {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return &response, true, nil
}

func protonJSONRequest(ctx context.Context, client *http.Client, method, url, appVersion, userAgent string, body io.Reader, out any) error {
	_, err := protonJSONRequestStatus(ctx, client, method, url, appVersion, userAgent, body, out)
	return err
}

func protonJSONRequestStatus(ctx context.Context, client *http.Client, method, url, appVersion, userAgent string, body io.Reader, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return 0, err
	}
	req.Header.Set("x-pm-appversion", appVersion)
	if userAgent != "" {
		req.Header.Set("User-Agent", userAgent)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	res, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() {
		_ = res.Body.Close()
	}()

	responseBody, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return res.StatusCode, fmt.Errorf("proton API request failed: %s: %s", res.Status, string(responseBody))
	}
	if out == nil {
		return res.StatusCode, nil
	}
	if err := json.Unmarshal(responseBody, out); err != nil {
		return res.StatusCode, err
	}
	return res.StatusCode, nil
}

func protonGenerateSignInURL(userCode string) ([]byte, string, error) {
	encryptionKey := make([]byte, 32)
	if _, err := rand.Read(encryptionKey); err != nil {
		return nil, "", err
	}

	payload := fmt.Sprintf("0:%s:%s:%s", userCode, base64.StdEncoding.EncodeToString(encryptionKey), protonWebAuthClientID)
	signInURL := protonAccountURL + "/desktop/login?app=drive&pv=3#payload=" + url.QueryEscape(payload)
	return encryptionKey, signInURL, nil
}

func protonParseForkKeyPassword(encryptionKey []byte, encryptedPayload string) (string, error) {
	payloadBlob, err := base64.StdEncoding.DecodeString(encryptedPayload)
	if err != nil {
		return "", err
	}
	if len(payloadBlob) < 12+16 {
		return "", errors.New("invalid Proton fork payload length")
	}

	nonce := payloadBlob[:12]
	tag := payloadBlob[len(payloadBlob)-16:]
	ciphertext := payloadBlob[12 : len(payloadBlob)-16]

	block, err := aes.NewCipher(encryptionKey)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	sealed := append(bytes.Clone(ciphertext), tag...)
	plaintext, err := gcm.Open(nil, nonce, sealed, protonForkAAD)
	if err != nil {
		return "", err
	}

	var payload protonForkPayload
	if err := json.Unmarshal(plaintext, &payload); err != nil {
		return "", err
	}
	if payload.KeyPassword == "" {
		return "", errors.New("proton fork payload did not include keyPassword")
	}
	return payload.KeyPassword, nil
}

func normalizeSaltedKeyPass(keyPassword string) string {
	if _, err := base64.StdEncoding.DecodeString(keyPassword); err == nil {
		return keyPassword
	}
	return base64.StdEncoding.EncodeToString([]byte(keyPassword))
}

func sleepContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
