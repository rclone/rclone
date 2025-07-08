// Build for protondrive for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || wasm

// Package protondrive provides a filesystem interface to Proton Drive
package protondrive
