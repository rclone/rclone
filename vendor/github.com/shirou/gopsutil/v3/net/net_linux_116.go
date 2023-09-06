//go:build go1.16
// +build go1.16

package net

import (
	"os"
)

func readDir(f *os.File, max int) ([]os.DirEntry, error) {
	return f.ReadDir(max)
}
