//go:build !windows

package oauthutil

// bindErrorHint returns extra guidance to append to an error from binding the
// local OAuth webserver. Only Windows currently has a useful hint to add.
func bindErrorHint(error) string {
	return ""
}
