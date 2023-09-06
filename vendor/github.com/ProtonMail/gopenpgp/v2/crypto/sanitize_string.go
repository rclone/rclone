package crypto

import "strings"

func sanitizeString(input string) string {
	return strings.ToValidUTF8(input, "\ufffd")
}
