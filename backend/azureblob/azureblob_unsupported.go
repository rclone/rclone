// Build for azureblob for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build freebsd netbsd openbsd plan9 solaris !go1.8

package azureblob
