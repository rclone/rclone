// ReadPassword for OSes which are not supported by golang.org/x/crypto/ssh/terminal
// See https://github.com/golang/go/issues/14441 - plan9
//     https://github.com/golang/go/issues/13085 - solaris

// +build solaris plan9

package config

// ReadPassword reads a password with echoing it to the terminal.
func ReadPassword() string {
	return ReadLine()
}
