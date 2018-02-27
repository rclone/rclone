package daemon

import (
	"os"
)

// AddCommand is wrapper on AddFlag and SetSigHandler functions.
func AddCommand(f Flag, sig os.Signal, handler SignalHandlerFunc) {
	if f != nil {
		AddFlag(f, sig)
	}
	if handler != nil {
		SetSigHandler(handler, sig)
	}
}

// Flag is the interface implemented by an object that has two state:
// 'set' and 'unset'.
type Flag interface {
	IsSet() bool
}

// BoolFlag returns new object that implements interface Flag and
// has state 'set' when var with the given address is true.
func BoolFlag(f *bool) Flag {
	return &boolFlag{f}
}

// StringFlag returns new object that implements interface Flag and
// has state 'set' when var with the given address equals given value of v.
func StringFlag(f *string, v string) Flag {
	return &stringFlag{f, v}
}

type boolFlag struct {
	b *bool
}

func (f *boolFlag) IsSet() bool {
	if f == nil {
		return false
	}
	return *f.b
}

type stringFlag struct {
	s *string
	v string
}

func (f *stringFlag) IsSet() bool {
	if f == nil {
		return false
	}
	return *f.s == f.v
}

var flags = make(map[Flag]os.Signal)

// Flags returns flags that was added by the function AddFlag.
func Flags() map[Flag]os.Signal {
	return flags
}

// AddFlag adds the flag and signal to the internal map.
func AddFlag(f Flag, sig os.Signal) {
	flags[f] = sig
}

// SendCommands sends active signals to the given process.
func SendCommands(p *os.Process) (err error) {
	for _, sig := range signals() {
		if err = p.Signal(sig); err != nil {
			return
		}
	}
	return
}

// ActiveFlags returns flags that has the state 'set'.
func ActiveFlags() (ret []Flag) {
	ret = make([]Flag, 0, 1)
	for f := range flags {
		if f.IsSet() {
			ret = append(ret, f)
		}
	}
	return
}

func signals() (ret []os.Signal) {
	ret = make([]os.Signal, 0, 1)
	for f, sig := range flags {
		if f.IsSet() {
			ret = append(ret, sig)
		}
	}
	return
}
