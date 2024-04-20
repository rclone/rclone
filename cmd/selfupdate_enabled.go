//go:build !noselfupdate

package cmd

// This constant must be in the `cmd` package rather than `cmd/selfupdate`
// to prevent build failure due to dependency loop.
const selfupdateEnabled = true
