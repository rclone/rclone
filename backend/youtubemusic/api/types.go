// Package api provides types used by the YouTube Music API.
package api

// OAuthTVAndLimitedDeviceRequest represents the JSON API object that's sent to the oauth API endpoint.
type OAuthTVAndLimitedDeviceRequest struct {
	Scope    string `json:"scope"`
	ClientID string `json:"client_id"`
}

// OAuthTVAndLimitedDeviceResponse represents the JSON API object that's received from the oauth API endpoint.
type OAuthTVAndLimitedDeviceResponse struct {
	DeviceCode      string `json:"device_code"`
	UserCode        string `json:"user_code"`
	ExpiresIn       int    `json:"expires_in"`
	Interval        int    `json:"interval"`
	VerificationURL string `json:"verification_url"`
	Error           string `json:"error"`
}

// OAuthTokenTVAndLimitedDeviceRequest represents the JSON API object that's sent to the oauth token API endpoint.
type OAuthTokenTVAndLimitedDeviceRequest struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	GrantType    string `json:"grant_type"`
	Code         string `json:"code"`
}

// OAuthTokenTVAndLimitedDeviceResponse represents the JSON API object that's received from the oauth token API endpoint.
type OAuthTokenTVAndLimitedDeviceResponse struct {
	Scope            string `json:"scope"`
	TokenType        string `json:"token_type"`
	AccessToken      string `json:"access_token"`
	RefreshToken     string `json:"refresh_token"`
	ExpiresIn        int    `json:"expires_in"`
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}
