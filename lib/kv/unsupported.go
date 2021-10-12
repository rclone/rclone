//go:build plan9 || js
// +build plan9 js

package kv

import (
	"context"

	"github.com/rclone/rclone/fs"
)

// DB represents a key-value database
type DB struct{}

// Supported returns true on supported OSes
func Supported() bool { return false }

// Start a key-value database
func Start(ctx context.Context, facility string, f fs.Fs) (*DB, error) {
	return nil, ErrUnsupported
}

// Get returns database for given filesystem and facility
func Get(f fs.Fs, facility string) *DB { return nil }

// Path returns database path
func (*DB) Path() string { return "UNSUPPORTED" }

// Do submits a key-value request and waits for results
func (*DB) Do(write bool, op Op) error {
	return ErrUnsupported
}

// Stop a database loop, optionally removing the file
func (*DB) Stop(remove bool) error {
	return ErrUnsupported
}

// Exit stops all databases
func Exit() {}
