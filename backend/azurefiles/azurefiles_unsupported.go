// Build for azurefiles for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js || wasm

// Package azurefiles provides an interface to Microsoft Azure Files
package azurefiles
