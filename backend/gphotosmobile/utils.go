package gphotosmobile

import (
	"strings"
)

// parseEmail extracts the email from auth data
func parseEmail(authData string) string {
	for param := range strings.SplitSeq(authData, "&") {
		if after, ok := strings.CutPrefix(param, "Email="); ok {
			email := after
			email = strings.ReplaceAll(email, "%40", "@")
			return email
		}
	}
	return ""
}

// parseLanguage extracts the language from auth data
func parseLanguage(authData string) string {
	for param := range strings.SplitSeq(authData, "&") {
		if after, ok := strings.CutPrefix(param, "lang="); ok {
			return after
		}
	}
	return ""
}
