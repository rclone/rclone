// Package youtubemusic provides the youtubemusic backend.
package youtubemusic

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rclone/rclone/backend/youtubemusic/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/rest"
	"github.com/skratchdot/open-golang/open"
	"golang.org/x/oauth2"
)

const (
	// The app for these API keys should be for TV and Limited-Input Device.
	rcloneClientID              = "861556708454-d6dlm3lh05idd8npek18k6be8ba3oc68.apps.googleusercontent.com" // TODO: update this
	rcloneEncryptedClientSecret = "aCVz_k_XoJc9gc3XuuDCeq2VzXsQFd4QTKsyF8ir2Bgnj5abr28JQw"                   // TODO: update this

	// These 2 scopes are the only YouTube Data API scopes available for TV and Limited-Input Device.
	// source: https://developers.google.com/youtube/v3/guides/auth/devices#allowedscopes
	scopeYoutubeReadWrite = "https://www.googleapis.com/auth/youtube"          // manage your YouTube account
	scopeYoutubeReadOnly  = "https://www.googleapis.com/auth/youtube.readonly" // view your YouTube account

	ytmusicDomain    = "https://music.youtube.com"
	ytmusicBaseAPI   = ytmusicDomain + "/youtubei/v1/"
	ytmusicParams    = "?alt=json"
	ytmusicParamsKey = "&key=AIzaSyC9XL3ZjWddXya6X74dJoCTL-WEYFDNX30"
	userAgent        = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:88.0) Gecko/20100101 Firefox/88.0"

	oauthCodeURL   = "https://www.youtube.com/o/oauth2/device/code"
	oauthTokenURL  = "https://oauth2.googleapis.com/token"
	oauthUserAgent = userAgent + " Cobalt/Version"
)

var (
	oauthScope string
	deviceCode string
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "youtube music",
		Prefix:      "ytmusic",
		Description: "YouTube Music",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Parse config into Options struct
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			switch config.State {
			case "":
				// Fill in the scopes
				if opt.ReadOnly {
					oauthScope = scopeYoutubeReadOnly
				} else {
					oauthScope = scopeYoutubeReadWrite
				}

				// Update client_id and client_secret if they are empty
				if val, _ := m.Get("client_id"); val == "" {
					m.Set("client_id", rcloneClientID)
				}
				if val, _ := m.Get("client_secret"); val == "" {
					m.Set("client_secret", obscure.MustReveal(rcloneEncryptedClientSecret))
				}

				// Post the response from oauthCodeURL
				clientID, _ := m.Get("client_id")
				oauthTVAndLimitedDevice, err := postOAuthTVAndLimitedDevice(ctx, api.OAuthTVAndLimitedDeviceRequest{
					Scope:    oauthScope,
					ClientID: clientID,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to postOAuthCodeURL: %w", err)
				}
				deviceCode = oauthTVAndLimitedDevice.DeviceCode

				// Open the verification URL in the browser
				url := fmt.Sprintf("%s?user_code=%s", oauthTVAndLimitedDevice.VerificationURL, oauthTVAndLimitedDevice.UserCode)
				open.Start(url)

				return fs.ConfigConfirm("config_auth_do", true, "config_init", fmt.Sprintf("Go to %s, finish the login flow and press Enter when done, Ctrl-C to abort", url))
			case "config_auth_do": // Continue the authentication process
				// Post the response from oauthTokenURL
				clientID, _ := m.Get("client_id")
				clientSecret, _ := m.Get("client_secret")
				oauthTokenTVAndLimitedDevice, err := postOAuthTokenTVAndLimitedDevice(ctx, api.OAuthTokenTVAndLimitedDeviceRequest{
					ClientID:     clientID,
					ClientSecret: clientSecret,
					GrantType:    "http://oauth.net/grant_type/device/1.0",
					Code:         deviceCode,
				})
				if err != nil {
					return nil, fmt.Errorf("failed to postOAuthTokenURL: %w", err)
				}

				// Save the token to the config file
				err = oauthutil.PutToken(name, m, &oauth2.Token{
					AccessToken:  oauthTokenTVAndLimitedDevice.AccessToken,
					RefreshToken: oauthTokenTVAndLimitedDevice.RefreshToken,
					TokenType:    oauthTokenTVAndLimitedDevice.TokenType,
					Expiry:       time.Now().Add(time.Duration(oauthTokenTVAndLimitedDevice.ExpiresIn) * time.Second),
				}, true)
				if err != nil {
					return nil, fmt.Errorf("error while saving token: %w", err)
				}

				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name:    "read_only",
			Default: false,
			Help: `Set to make the YouTube Music backend read only.

If you choose read only then rclone will only request read only access
to your music, otherwise rclone will request full access.`,
		}, {
			Name: "playlist-id",
			Help: "ID of the playlist to sync. Can be found in the playlist's URL.",
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	ReadOnly bool `config:"read_only"`

	// ReadSize        bool                 `config:"read_size"`
	// StartYear       int                  `config:"start_year"`
	// IncludeArchived bool                 `config:"include_archived"`
	// Enc             encoder.MultiEncoder `config:"encoding"`
	// BatchMode       string               `config:"batch_mode"`
	// BatchSize       int                  `config:"batch_size"`
	// BatchTimeout    fs.Duration          `config:"batch_timeout"`
}

// ------------------------------------------------------------

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// postOAuthTVAndLimitedDevice sends a POST request to the oauthCodeURL and returns the response.
func postOAuthTVAndLimitedDevice(ctx context.Context, reqData api.OAuthTVAndLimitedDeviceRequest) (resp api.OAuthTVAndLimitedDeviceResponse, err error) {
	// Create a new HTTP client with the provided context.
	httpClient := fshttp.NewClient(ctx)

	// Marshal the request data into JSON.
	reqStr, err := json.Marshal(reqData)
	if err != nil {
		return resp, fmt.Errorf("failed to marshal reqData: %w", err)
	}

	// Send a POST request to the oauthCodeURL with the request data.
	respHTTP, err := httpClient.Post(oauthCodeURL, "application/json", bytes.NewBuffer(reqStr))
	if err != nil {
		return resp, fmt.Errorf("failed to get oauth code: %w", err)
	}
	defer fs.CheckClose(respHTTP.Body, &err)

	// Read the response body.
	respBody, err := rest.ReadBody(respHTTP)
	if err != nil {
		return resp, fmt.Errorf("failed to read respBody: %w", err)
	}

	// Unmarshal the response body into a struct.
	response := new(api.OAuthTVAndLimitedDeviceResponse)
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return resp, fmt.Errorf("failed to parse respBody: %w", err)
	}
	if response.Error != "" {
		return resp, fmt.Errorf("failed to get oauth code: %s", respBody)
	}

	return *response, nil
}

// postOAuthTokenTVAndLimitedDevice sends a POST request to the oauthTokenURL and returns the response.
func postOAuthTokenTVAndLimitedDevice(ctx context.Context, reqData api.OAuthTokenTVAndLimitedDeviceRequest) (resp api.OAuthTokenTVAndLimitedDeviceResponse, err error) {
	// Create a new HTTP client with the provided context.
	httpClient := fshttp.NewClient(ctx)

	// Marshal the request data into JSON.
	reqStr, err := json.Marshal(reqData)
	if err != nil {
		return resp, fmt.Errorf("failed to marshal reqData: %w", err)
	}

	// Send a POST request to the oauthTokenURL with the request data.
	respHTTP, err := httpClient.Post(oauthTokenURL, "application/json", bytes.NewBuffer(reqStr))
	if err != nil {
		return resp, fmt.Errorf("failed to get oauth token: %w", err)
	}
	defer fs.CheckClose(respHTTP.Body, &err)

	// Read the response body.
	respBody, err := rest.ReadBody(respHTTP)
	if err != nil {
		return resp, fmt.Errorf("failed to read respBody: %w", err)
	}

	// Unmarshal the response body into a struct.
	response := new(api.OAuthTokenTVAndLimitedDeviceResponse)
	err = json.Unmarshal(respBody, &response)
	if err != nil {
		return resp, fmt.Errorf("failed to parse respBody: %w", err)
	}
	if response.Error != "" {
		return resp, fmt.Errorf("failed to get oauth token: %s", respBody)
	}

	return *response, nil
}
