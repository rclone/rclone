//+build !windows

package cmd

import "os"

func initTerminal() error {
	return nil
}

func writeToTerminal(b []byte) {
	_, _ = os.Stdout.Write(b)
}
