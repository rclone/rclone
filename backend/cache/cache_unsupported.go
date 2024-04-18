// Build for cache for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js

// Package cache implements a virtual provider to cache existing remotes.
package cache
