package fstestutil

import (
	"flag"
	"log"
	"strconv"

	"bazil.org/fuse"
)

type flagDebug bool

var debug flagDebug

var _ = flag.Value(&debug)

func (f *flagDebug) IsBoolFlag() bool {
	return true
}

func nop(msg interface{}) {}

func (f *flagDebug) Set(s string) error {
	v, err := strconv.ParseBool(s)
	if err != nil {
		return err
	}
	*f = flagDebug(v)
	if v {
		fuse.Debug = logMsg
	} else {
		fuse.Debug = nop
	}
	return nil
}

func (f *flagDebug) String() string {
	return strconv.FormatBool(bool(*f))
}

func logMsg(msg interface{}) {
	log.Printf("FUSE: %s\n", msg)
}

func init() {
	flag.Var(&debug, "fuse.debug", "log FUSE processing details")
}

// DebugByDefault changes the default of the `-fuse.debug` flag to
// true.
//
// This package registers a command line flag `-fuse.debug` and when
// run with that flag (and activated inside the tests), logs FUSE
// debug messages.
//
// This is disabled by default, as most callers probably won't care
// about FUSE details. Use DebugByDefault for tests where you'd
// normally be passing `-fuse.debug` all the time anyway.
//
// Call from an init function.
func DebugByDefault() {
	f := flag.Lookup("fuse.debug")
	f.DefValue = "true"
	f.Value.Set(f.DefValue)
}
