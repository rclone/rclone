// Build for azureblob for unsupported platforms to stop go complaining
// about "no buildable Go source files "

// +build plan9 solaris js !go1.14

package azureblob
