// Build for azureblob for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || solaris || js || wasm

// Package azureblob provides an interface to the Microsoft Azure blob object storage system
package azureblob
