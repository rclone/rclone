// Package internal contains internal methods and constants.
package internal

import (
	"strings"

	"github.com/ProtonMail/gopenpgp/v2/constants"
)

func Canonicalize(text string) string {
	return strings.ReplaceAll(strings.ReplaceAll(text, "\r\n", "\n"), "\n", "\r\n")
}

func TrimEachLine(text string) string {
	lines := strings.Split(text, "\n")

	for i := range lines {
		lines[i] = strings.TrimRight(lines[i], " \t\r")
	}

	return strings.Join(lines, "\n")
}

// CreationTimeOffset stores the amount of seconds that a signature may be
// created in the future, to compensate for clock skew.
const CreationTimeOffset = int64(60 * 60 * 24 * 2)

// ArmorHeaders is a map of default armor headers.
var ArmorHeaders = map[string]string{
	"Version": constants.ArmorHeaderVersion,
	"Comment": constants.ArmorHeaderComment,
}
