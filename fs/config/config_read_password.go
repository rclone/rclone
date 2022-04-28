// ReadPassword for OSes which are supported by golang.org/x/term
// See https://github.com/golang/go/issues/14441 - plan9

//go:build !plan9
// +build !plan9

package config

import (
	"fmt"
	"log"
	"os"

	"github.com/rclone/rclone/lib/terminal"
)

// ReadPassword reads a password without echoing it to the terminal.
func ReadPassword() string {
	stdin := int(os.Stdin.Fd())
	if !terminal.IsTerminal(stdin) {
		return ReadLine()
	}
	line, err := terminal.ReadPassword(stdin)
	_, _ = fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("Failed to read password: %v", err)
	}
	return string(line)
}
