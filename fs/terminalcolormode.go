package fs

import (
	"fmt"
	"strings"
)

// TerminalColorMode describes how ANSI codes should be handled
type TerminalColorMode byte

// TerminalColorMode constants
const (
	TerminalColorModeAuto TerminalColorMode = iota
	TerminalColorModeNever
	TerminalColorModeAlways
)

var terminalColorModeToString = []string{
	TerminalColorModeAuto:   "AUTO",
	TerminalColorModeNever:  "NEVER",
	TerminalColorModeAlways: "ALWAYS",
}

// String converts a TerminalColorMode to a string
func (m TerminalColorMode) String() string {
	if m >= TerminalColorMode(len(terminalColorModeToString)) {
		return fmt.Sprintf("TerminalColorMode(%d)", m)
	}
	return terminalColorModeToString[m]
}

// Set a TerminalColorMode
func (m *TerminalColorMode) Set(s string) error {
	for n, name := range terminalColorModeToString {
		if s != "" && name == strings.ToUpper(s) {
			*m = TerminalColorMode(n)
			return nil
		}
	}
	return fmt.Errorf("unknown terminal color mode %q", s)
}

// Type of TerminalColorMode
func (m TerminalColorMode) Type() string {
	return "string"
}

// UnmarshalJSON converts a string/integer in JSON to a TerminalColorMode
func (m *TerminalColorMode) UnmarshalJSON(in []byte) error {
	return UnmarshalJSONFlag(in, m, func(i int64) error {
		if i < 0 || i >= int64(len(terminalColorModeToString)) {
			return fmt.Errorf("out of range terminal color mode %d", i)
		}
		*m = (TerminalColorMode)(i)
		return nil
	})
}
