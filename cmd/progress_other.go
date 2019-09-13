//+build !windows

package cmd

func init() {
	// Default terminal is VT100 for non Windows
	initTerminal = initTerminalVT100
	writeToTerminal = writeToTerminalVT100
}
