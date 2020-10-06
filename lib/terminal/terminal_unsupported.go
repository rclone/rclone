//+build js

package terminal

import "errors"

// GetSize reads the dimensions of the current terminal or returns a
// sensible default
func GetSize() (w, h int) {
	return 80, 25
}

// IsTerminal returns whether the fd passed in is a terminal or not
func IsTerminal(fd int) bool {
	return false
}

// ReadPassword reads a line of input from a terminal without local echo. This
// is commonly used for inputting passwords and other sensitive data. The slice
// returned does not include the \n.
func ReadPassword(fd int) ([]byte, error) {
	return nil, errors.New("can't read password")
}

// WriteTerminalTitle writes a string to the terminal title
func WriteTerminalTitle(title string) {
	// Since there's nothing to return, this is a NOOP
}
