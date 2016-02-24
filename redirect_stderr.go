// Log the panic to the log file - for oses which can't do this

// +build !windows,!darwin,!dragonfly,!freebsd,!linux,!nacl,!netbsd,!openbsd

package main

import (
	"log"
	"os"
)

// redirectStderr to the file passed in
func redirectStderr(f *os.File) {
	log.Printf("Can't redirect stderr to file")
}
