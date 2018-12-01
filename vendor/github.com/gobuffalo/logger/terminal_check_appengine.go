// +build appengine

package logger

import (
	"io"
)

func checkIfTerminal(w io.Writer) bool {
	return true
}
