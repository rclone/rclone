//+build !windows

package cmd

import "os"

func writeToTerminal(b []byte) {
	_, _ = os.Stdout.Write(b)
}
