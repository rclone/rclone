package funambol

// O2 Cloud macOS 4.0.0 exposes these application credentials through
// Brand::getOAuth2ClientId() and Brand::getOAuth2ClientSecret() in libbrand.dylib.
// Installer: https://cloud.o2online.es/macos/mac-os-app-installer.dmg
const (
	o2OAuthClientID              = "21ce2068-4966-45dc-8b7f-a4a25a3a462f"
	o2OAuthEncryptedClientSecret = "dgzrt49ynXtZiGV_JTytJ-jJOzEiQZ3XB2-vNfL23vMHko4bjfl3feG2XxRqLtfpr9csqA"
)

// setOAuthDefaults keeps the built-in application pair on its registered endpoints.
func (opt *Options) setOAuthDefaults() {
	if opt.Pass == "" && opt.ClientID == "" && opt.ClientSecret == "" && opt.Endpoint == defaultEndpoint &&
		o2AuthURL == o2OAuthAuthorize && o2TokenURL == o2OAuthToken {
		opt.ClientID = o2OAuthClientID
		opt.ClientSecret = o2OAuthEncryptedClientSecret
		if opt.OAuthPlatform == "" {
			opt.OAuthPlatform = "macos"
		}
	}
	if opt.OAuthPlatform == "" {
		opt.OAuthPlatform = "windows"
	}
}
