// Build for sftp for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build plan9 !go1.8

package sftp
