// Build for nfsmount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build !darwin || cmount
// +build !darwin cmount

// Package nfsmount implements mount command using NFS, not needed on most platforms
package nfsmount
