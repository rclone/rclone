//go:build go1.21

package log

import "testing"

func init() {
	if testing.Testing() {
		DefaultTimeFormatter = TimeFormatSecondsSinceInit
	}
}
