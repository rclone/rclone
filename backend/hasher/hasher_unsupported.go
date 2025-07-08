// Build for hasher for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || wasm

// Package hasher provides a SFTP filesystem interface
package hasher
