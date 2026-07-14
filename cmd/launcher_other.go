//go:build !windows

package cmd

func launchedFromExplorer() bool {
	return false
}
