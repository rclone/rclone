package funambol

// O2 Cloud macOS 4.0.0 exposes these application credentials through
// Brand::getOAuth2ClientId() and Brand::getOAuth2ClientSecret() in libbrand.dylib.
// Installer: https://cloud.o2online.es/macos/mac-os-app-installer.dmg
const (
	o2OAuthClientID              = "21ce2068-4966-45dc-8b7f-a4a25a3a462f"
	o2OAuthEncryptedClientSecret = "dgzrt49ynXtZiGV_JTytJ-jJOzEiQZ3XB2-vNfL23vMHko4bjfl3feG2XxRqLtfpr9csqA"
)

// These are the application credentials of the Movistar Cloud Windows
// desktop client, as used by https://github.com/uniextra/MovistarWebDAVProxy.
//
// TODO: confirm them against the official Windows installer.
const (
	movistarOAuthClientID              = "b99e5095-7a36-42c1-83b6-6f05d0f13ea1"
	movistarOAuthEncryptedClientSecret = "V3BskIMOxPtCKgkc0HEcJyfoQd1ksnMccI8tTNYz_EtpwyPUoh6YnBYFpGjGFKPZ2Bf8Sw"
)

// setOAuthDefaults keeps each built-in application pair on its registered endpoints.
func (opt *Options) setOAuthDefaults() {
	if opt.Pass == "" && opt.ClientID == "" && opt.ClientSecret == "" {
		switch {
		case opt.Endpoint == defaultEndpoint && o2AuthURL == o2OAuthAuthorize && o2TokenURL == o2OAuthToken:
			opt.ClientID = o2OAuthClientID
			opt.ClientSecret = o2OAuthEncryptedClientSecret
			if opt.OAuthPlatform == "" {
				opt.OAuthPlatform = "macos"
			}
		case opt.isMovistar() && movistarAuthURL == movistarOAuthAuthorize && movistarTokenURL == movistarOAuthToken:
			opt.ClientID = movistarOAuthClientID
			opt.ClientSecret = movistarOAuthEncryptedClientSecret
		}
	}
	if opt.OAuthPlatform == "" {
		opt.OAuthPlatform = "windows"
	}
}
