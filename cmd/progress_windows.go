//+build windows

package cmd

import (
	"fmt"
	"os"
	"syscall"

	ansiterm "github.com/Azure/go-ansiterm"
	"github.com/Azure/go-ansiterm/winterm"
	"github.com/pkg/errors"
)

var (
	ansiParser *ansiterm.AnsiParser
)

func initTerminal() error {
	winEventHandler := winterm.CreateWinEventHandler(os.Stdout.Fd(), os.Stdout)
	if winEventHandler == nil {
		err := syscall.GetLastError()
		if err == nil {
			err = errors.New("initialization failed")
		}
		return errors.Wrap(err, "windows terminal")
	}
	ansiParser = ansiterm.CreateParser("Ground", winEventHandler)
	return nil
}

func writeToTerminal(b []byte) {
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
