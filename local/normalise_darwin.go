// +build darwin

package local

import (
	"golang.org/x/text/unicode/norm"
)

// normString normalises the remote name as some OS X denormalises UTF-8 when storing it to disk
func normString(remote string) string {
	return norm.NFC.String(remote)
}
