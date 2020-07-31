//+build !js

package terminal

import (
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

// GetSize reads the dimensions of the current terminal or returns a
// sensible default
func GetSize() (w, h int) {
	w, h, err := terminal.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 25
	}
	return w, h
}

// IsTerminal returns whether the fd passed in is a terminal or not
func IsTerminal(fd int) bool {
	return terminal.IsTerminal(fd)
}

// ReadPassword reads a line of input from a terminal without local echo. This
// is commonly used for inputting passwords and other sensitive data. The slice
// returned does not include the \n.
func ReadPassword(fd int) ([]byte, error) {
	return terminal.ReadPassword(fd)
}
