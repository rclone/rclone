// Package terminal provides VT100 terminal codes and a windows
// implementation of that.
package terminal

import (
	"io"
	"os"
	"runtime"
	"sync"

	colorable "github.com/mattn/go-colorable"
)

// VT100 codes
const (
	EraseLine         = "\x1b[2K"
	MoveToStartOfLine = "\x1b[1G"
	MoveUp            = "\x1b[1A"

	Reset      = "\x1b[0m"
	Bright     = "\x1b[1m"
	Dim        = "\x1b[2m"
	Underscore = "\x1b[4m"
	Blink      = "\x1b[5m"
	Reverse    = "\x1b[7m"
	Hidden     = "\x1b[8m"

	BlackFg   = "\x1b[30m"
	RedFg     = "\x1b[31m"
	GreenFg   = "\x1b[32m"
	YellowFg  = "\x1b[33m"
	BlueFg    = "\x1b[34m"
	MagentaFg = "\x1b[35m"
	CyanFg    = "\x1b[36m"
	WhiteFg   = "\x1b[37m"

	BlackBg   = "\x1b[40m"
	RedBg     = "\x1b[41m"
	GreenBg   = "\x1b[42m"
	YellowBg  = "\x1b[43m"
	BlueBg    = "\x1b[44m"
	MagentaBg = "\x1b[45m"
	CyanBg    = "\x1b[46m"
	WhiteBg   = "\x1b[47m"

	HiBlackFg   = "\x1b[90m"
	HiRedFg     = "\x1b[91m"
	HiGreenFg   = "\x1b[92m"
	HiYellowFg  = "\x1b[93m"
	HiBlueFg    = "\x1b[94m"
	HiMagentaFg = "\x1b[95m"
	HiCyanFg    = "\x1b[96m"
	HiWhiteFg   = "\x1b[97m"

	HiBlackBg   = "\x1b[100m"
	HiRedBg     = "\x1b[101m"
	HiGreenBg   = "\x1b[102m"
	HiYellowBg  = "\x1b[103m"
	HiBlueBg    = "\x1b[104m"
	HiMagentaBg = "\x1b[105m"
	HiCyanBg    = "\x1b[106m"
	HiWhiteBg   = "\x1b[107m"

	ChangeTitle = "\033]0;"
	BEL         = "\007"
)

var (
	// make sure that start is only called once
	once sync.Once
)

// Start the terminal - must be called before use
func Start() {
	once.Do(func() {
		f := os.Stdout
		if !IsTerminal(int(f.Fd())) {
			// If stdout not a tty then remove escape codes
			Out = colorable.NewNonColorable(f)
		} else if runtime.GOOS == "windows" && os.Getenv("TERM") != "" {
			// If TERM is set just use stdout
			Out = f
		} else {
			Out = colorable.NewColorable(f)
		}
	})
}

// WriteString writes the string passed in to the terminal
func WriteString(s string) {
	Write([]byte(s))
}

// Out is an io.Writer which can be used to write to the terminal
// e.g. for use with fmt.Fprintf(terminal.Out, "terminal fun: %d\n", n)
var Out io.Writer

// Write sends out to the VT100 terminal.
// It will initialise the terminal if this is the first call.
func Write(out []byte) {
	Start()
	_, _ = Out.Write(out)
}
