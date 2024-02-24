// Build for nfsmount for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build !unix
// +build !unix

// Package nfsmount implements mount command using NFS.
package nfsmount
