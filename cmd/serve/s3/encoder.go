package s3

import "strings"

// From: minio/cmd/api-utils.go
// License: AGPL-3.0

func shouldEscape(c byte) bool {
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}

	switch c {
	case '-', '_', '.', '/', '*':
		return false
	}
	return true
}

// s3URLEncode is based on Golang's url.QueryEscape() code,
// while considering some S3 exceptions:
//   - Avoid encoding '/' and '*'
//   - Force encoding of '~'
func s3URLEncode(s string) string {
	spaceCount, hexCount := 0, 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if shouldEscape(c) {
			if c == ' ' {
				spaceCount++
			} else {
				hexCount++
			}
		}
	}

	if spaceCount == 0 && hexCount == 0 {
		return s
	}

	var buf [64]byte
	var t []byte

	required := len(s) + 2*hexCount
	if required <= len(buf) {
		t = buf[:required]
	} else {
		t = make([]byte, required)
	}

	if hexCount == 0 {
		copy(t, s)
		for i := 0; i < len(s); i++ {
			if s[i] == ' ' {
				t[i] = '+'
			}
		}
		return string(t)
	}

	j := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c == ' ':
			t[j] = '+'
			j++
		case shouldEscape(c):
			t[j] = '%'
			t[j+1] = "0123456789ABCDEF"[c>>4]
			t[j+2] = "0123456789ABCDEF"[c&15]
			j += 3
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}

// s3EncodeName encodes string in response when encodingType is specified in AWS S3 requests.
func s3EncodeName(name string, encodingType string) (result string) {
	// Quick path to exit
	if encodingType == "" {
		return name
	}
	encodingType = strings.ToLower(encodingType)
	switch encodingType {
	case "url":
		return s3URLEncode(name)
	}
	return name
}
