// ReadPassword for OSes which are not supported by golang.org/x/term
// See https://github.com/golang/go/issues/14441 - plan9

//go:build plan9
// +build plan9

package config

// ReadPassword reads a password with echoing it to the terminal.
func ReadPassword() string {
	return ReadLine()
}
