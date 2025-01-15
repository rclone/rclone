// Build for oracleobjectstorage for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || solaris || js

// Package oracleobjectstorage provides an interface to the OCI object storage system.
package oracleobjectstorage
