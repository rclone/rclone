// Build for azureblob for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || solaris || js || !go1.18
// +build plan9 solaris js !go1.18

package azureblob
