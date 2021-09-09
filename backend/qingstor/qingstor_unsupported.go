// Build for unsupported platforms to stop go complaining
// about "no buildable Go source files "

//go:build plan9 || js
// +build plan9 js

package qingstor
