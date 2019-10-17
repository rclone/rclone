// Define the internal rc functions

package rc

import (
	"context"
	"os"
	"runtime"
	"time"

	"github.com/pkg/errors"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/version"
	"github.com/rclone/rclone/lib/atexit"
)

func init() {
	Add(Call{
		Path:         "rc/noopauth",
		AuthRequired: true,
		Fn:           rcNoop,
		Title:        "Echo the input to the output parameters requiring auth",
		Help: `
This echoes the input parameters to the output parameters for testing
purposes.  It can be used to check that rclone is still alive and to
check that parameter passing is working properly.`,
	})
	Add(Call{
		Path:  "rc/noop",
		Fn:    rcNoop,
		Title: "Echo the input to the output parameters",
		Help: `
This echoes the input parameters to the output parameters for testing
purposes.  It can be used to check that rclone is still alive and to
check that parameter passing is working properly.`,
	})
}

// Echo the input to the output parameters
func rcNoop(ctx context.Context, in Params) (out Params, err error) {
	return in, nil
}

func init() {
	Add(Call{
		Path:  "rc/error",
		Fn:    rcError,
		Title: "This returns an error",
		Help: `
This returns an error with the input as part of its error string.
Useful for testing error handling.`,
	})
}

// Return an error regardless
func rcError(ctx context.Context, in Params) (out Params, err error) {
	return nil, errors.Errorf("arbitrary error on input %+v", in)
}

func init() {
	Add(Call{
		Path:  "rc/list",
		Fn:    rcList,
		Title: "List all the registered remote control commands",
		Help: `
This lists all the registered remote control commands as a JSON map in
the commands response.`,
	})
}

// List the registered commands
func rcList(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	out["commands"] = Calls.List()
	return out, nil
}

func init() {
	Add(Call{
		Path:  "core/pid",
		Fn:    rcPid,
		Title: "Return PID of current process",
		Help: `
This returns PID of current process.
Useful for stopping rclone process.`,
	})
}

// Return PID of current process
func rcPid(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	out["pid"] = os.Getpid()
	return out, nil
}

func init() {
	Add(Call{
		Path:  "core/memstats",
		Fn:    rcMemStats,
		Title: "Returns the memory statistics",
		Help: `
This returns the memory statistics of the running program.  What the values mean
are explained in the go docs: https://golang.org/pkg/runtime/#MemStats

The most interesting values for most people are:

* HeapAlloc: This is the amount of memory rclone is actually using
* HeapSys: This is the amount of memory rclone has obtained from the OS
* Sys: this is the total amount of memory requested from the OS
  * It is virtual memory so may include unused memory
`,
	})
}

// Return the memory statistics
func rcMemStats(ctx context.Context, in Params) (out Params, err error) {
	out = make(Params)
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	out["Alloc"] = m.Alloc
	out["TotalAlloc"] = m.TotalAlloc
	out["Sys"] = m.Sys
	out["Mallocs"] = m.Mallocs
	out["Frees"] = m.Frees
	out["HeapAlloc"] = m.HeapAlloc
	out["HeapSys"] = m.HeapSys
	out["HeapIdle"] = m.HeapIdle
	out["HeapInuse"] = m.HeapInuse
	out["HeapReleased"] = m.HeapReleased
	out["HeapObjects"] = m.HeapObjects
	out["StackInuse"] = m.StackInuse
	out["StackSys"] = m.StackSys
	out["MSpanInuse"] = m.MSpanInuse
	out["MSpanSys"] = m.MSpanSys
	out["MCacheInuse"] = m.MCacheInuse
	out["MCacheSys"] = m.MCacheSys
	out["BuckHashSys"] = m.BuckHashSys
	out["GCSys"] = m.GCSys
	out["OtherSys"] = m.OtherSys
	return out, nil
}

func init() {
	Add(Call{
		Path:  "core/gc",
		Fn:    rcGc,
		Title: "Runs a garbage collection.",
		Help: `
This tells the go runtime to do a garbage collection run.  It isn't
necessary to call this normally, but it can be useful for debugging
memory problems.
`,
	})
}

// Do a garbage collection run
func rcGc(ctx context.Context, in Params) (out Params, err error) {
	runtime.GC()
	return nil, nil
}

func init() {
	Add(Call{
		Path:  "core/version",
		Fn:    rcVersion,
		Title: "Shows the current version of rclone and the go runtime.",
		Help: `
This shows the current version of go and the go runtime
- version - rclone version, eg "v1.44"
- decomposed - version number as [major, minor, patch, subpatch]
    - note patch and subpatch will be 999 for a git compiled version
- isGit - boolean - true if this was compiled from the git version
- os - OS in use as according to Go
- arch - cpu architecture in use according to Go
- goVersion - version of Go runtime in use

`,
	})
}

// Return version info
func rcVersion(ctx context.Context, in Params) (out Params, err error) {
	decomposed, err := version.New(fs.Version)
	if err != nil {
		return nil, err
	}
	out = Params{
		"version":    fs.Version,
		"decomposed": decomposed,
		"isGit":      decomposed.IsGit(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
		"goVersion":  runtime.Version(),
	}
	return out, nil
}

func init() {
	Add(Call{
		Path:  "core/obscure",
		Fn:    rcObscure,
		Title: "Obscures a string passed in.",
		Help: `
Pass a clear string and rclone will obscure it for the config file:
- clear - string

Returns
- obscured - string
`,
	})
}

// Return obscured string
func rcObscure(ctx context.Context, in Params) (out Params, err error) {
	clear, err := in.GetString("clear")
	if err != nil {
		return nil, err
	}
	obscured, err := obscure.Obscure(clear)
	if err != nil {
		return nil, err
	}
	out = Params{
		"obscured": obscured,
	}
	return out, nil
}

func init() {
	Add(Call{
		Path:  "core/quit",
		Fn:    rcQuit,
		Title: "Terminates the app.",
		Help: `
(optional) Pass an exit code to be used for terminating the app:
- exitCode - int
`,
	})
}

// Terminates app
func rcQuit(ctx context.Context, in Params) (out Params, err error) {
	code, err := in.GetInt64("exitCode")

	if IsErrParamInvalid(err) {
		return nil, err
	}
	if IsErrParamNotFound(err) {
		code = 0
	}
	exitCode := int(code)

	go func(exitCode int) {
		time.Sleep(time.Millisecond * 1500)
		atexit.Run()
		os.Exit(exitCode)
	}(exitCode)

	return nil, nil
}
