// Build for cryptomator for unsupported platforms to stop go complaining
// about "undefined xorBlock" in github.com/jacobsa/crypto cmac/hash.go 

//go:build js
// +build js

package cryptomator
