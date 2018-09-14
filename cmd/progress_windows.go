//+build windows

package cmd

import (
	"fmt"
	"os"
	"sync"

	ansiterm "github.com/Azure/go-ansiterm"
	"github.com/Azure/go-ansiterm/winterm"
)

var (
	initAnsiParser sync.Once
	ansiParser     *ansiterm.AnsiParser
)

func writeToTerminal(b []byte) {
	initAnsiParser.Do(func() {
		winEventHandler := winterm.CreateWinEventHandler(os.Stdout.Fd(), os.Stdout)
		ansiParser = ansiterm.CreateParser("Ground", winEventHandler)
	})
	// Remove all non-ASCII characters until this is fixed
	// https://github.com/Azure/go-ansiterm/issues/26
	r := []rune(string(b))
	for i := range r {
		if r[i] >= 127 {
			r[i] = '.'
		}
	}
	b = []byte(string(r))
	_, err := ansiParser.Parse(b)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "\n*** Error from ANSI parser: %v\n", err)
	}
}
