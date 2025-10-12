//go:build !windows

package local

import (
	"time"
)

const haveSetBTime = false

// setBTime changes the birth time of the file passed in
func setBTime(name string, btime time.Time) error {
	// Does nothing
	return nil
}

// lsetBTime changes the birth time of the link passed in
func lsetBTime(name string, btime time.Time) error {
	// Does nothing
	return nil
}
