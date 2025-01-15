//go:build !js

package terminal

import (
	"fmt"
	"os"

	"golang.org/x/term"
)

// GetSize reads the dimensions of the current terminal or returns a
// sensible default
func GetSize() (w, h int) {
	w, h, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		w, h = 80, 25
	}
	return w, h
}

// IsTerminal returns whether the fd passed in is a terminal or not
func IsTerminal(fd int) bool {
	return term.IsTerminal(fd)
}

// ReadPassword reads a line of input from a terminal without local echo. This
// is commonly used for inputting passwords and other sensitive data. The slice
// returned does not include the \n.
func ReadPassword(fd int) ([]byte, error) {
	return term.ReadPassword(fd)
}

// WriteTerminalTitle writes a string to the terminal title
func WriteTerminalTitle(title string) {
	fmt.Print(ChangeTitle + title + BEL)
}
