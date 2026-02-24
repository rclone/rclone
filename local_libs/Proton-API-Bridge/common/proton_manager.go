package common

import (
	"github.com/rclone/go-proton-api"
)

func getProtonManager(appVersion string, userAgent string) *proton.Manager {
	/* Notes on API calls: if the app version is not specified, the api calls will be rejected. */
	options := []proton.Option{
		proton.WithAppVersion(appVersion),
		proton.WithUserAgent(userAgent),
	}
	m := proton.New(options...)

	return m
}
