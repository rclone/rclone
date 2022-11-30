package fs

import (
	"fmt"
	"strings"
)

type TerminalColorMode byte

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

func (m TerminalColorMode) String() string {
	if m >= TerminalColorMode(len(terminalColorModeToString)) {
		return fmt.Sprintf("TerminalColorMode(%d)", m)
	}
	return terminalColorModeToString[m]
}

func (m *TerminalColorMode) Set(s string) error {
	for n, name := range terminalColorModeToString {
		if s != "" && name == strings.ToUpper(s) {
			*m = TerminalColorMode(n)
			return nil
		}
	}
	return fmt.Errorf("unknown terminal color mode %q", s)
}

func (m TerminalColorMode) Type() string {
	return "string"
}

func (m *TerminalColorMode) UnmarshalJSON(in []byte) error {
	return UnmarshalJSONFlag(in, m, func(i int64) error {
		if i < 0 || i >= int64(len(terminalColorModeToString)) {
			return fmt.Errorf("out of range terminal color mode %d", i)
		}
		*m = (TerminalColorMode)(i)
		return nil
	})
}
