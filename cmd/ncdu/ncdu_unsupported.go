// Build for ncdu for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js || aix
// +build plan9 js aix

package ncdu
