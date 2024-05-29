package fs

// TerminalColorMode describes how ANSI codes should be handled
type TerminalColorMode = Enum[terminalColorModeChoices]

// TerminalColorMode constants
const (
	TerminalColorModeAuto TerminalColorMode = iota
	TerminalColorModeNever
	TerminalColorModeAlways
)

type terminalColorModeChoices struct{}

func (terminalColorModeChoices) Choices() []string {
	return []string{
		TerminalColorModeAuto:   "AUTO",
		TerminalColorModeNever:  "NEVER",
		TerminalColorModeAlways: "ALWAYS",
	}
}
