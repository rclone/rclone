// ReadPassword for OSes which are supported by golang.org/x/crypto/ssh/terminal
// See https://github.com/golang/go/issues/14441 - plan9
//     https://github.com/golang/go/issues/13085 - solaris

// +build !solaris,!plan9

package config

import (
	"fmt"
	"log"
	"os"

	"golang.org/x/crypto/ssh/terminal"
)

// ReadPassword reads a password without echoing it to the terminal.
func ReadPassword() string {
	line, err := terminal.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr)
	if err != nil {
		log.Fatalf("Failed to read password: %v", err)
	}
	return string(line)
}
