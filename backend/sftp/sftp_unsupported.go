// Build for sftp for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9

// Package sftp provides a filesystem interface using github.com/pkg/sftp
package sftp
