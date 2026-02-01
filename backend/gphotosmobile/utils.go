package gphotosmobile

import (
	"crypto/sha1"
	"encoding/base64"
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

// calculateSHA1 calculates the SHA1 hash of data and returns (hash_bytes, hash_base64)
func calculateSHA1(data []byte) ([]byte, string) {
	h := sha1.New()
	h.Write(data)
	hashBytes := h.Sum(nil)
	hashB64 := base64.StdEncoding.EncodeToString(hashBytes)
	return hashBytes, hashB64
}
