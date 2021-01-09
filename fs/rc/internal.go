// Define the internal rc functions

package rc

import (
	"context"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/coreos/go-semver/semver"
	"github.com/pkg/errors"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/obscure"
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

- version - rclone version, e.g. "v1.53.0"
- decomposed - version number as [major, minor, patch]
- isGit - boolean - true if this was compiled from the git version
- isBeta - boolean - true if this is a beta version
- os - OS in use as according to Go
- arch - cpu architecture in use according to Go
- goVersion - version of Go runtime in use

`,
	})
}

// Return version info
func rcVersion(ctx context.Context, in Params) (out Params, err error) {
	version, err := semver.NewVersion(fs.Version[1:])
	if err != nil {
		return nil, err
	}
	out = Params{
		"version":    fs.Version,
		"decomposed": version.Slice(),
		"isGit":      strings.HasSuffix(fs.Version, "-DEV"),
		"isBeta":     version.PreRelease != "",
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

func init() {
	Add(Call{
		Path:  "debug/set-mutex-profile-fraction",
		Fn:    rcSetMutexProfileFraction,
		Title: "Set runtime.SetMutexProfileFraction for mutex profiling.",
		Help: `
SetMutexProfileFraction controls the fraction of mutex contention
events that are reported in the mutex profile. On average 1/rate
events are reported. The previous rate is returned.

To turn off profiling entirely, pass rate 0. To just read the current
rate, pass rate < 0. (For n>1 the details of sampling may change.)

Once this is set you can look use this to profile the mutex contention:

    go tool pprof http://localhost:5572/debug/pprof/mutex

Parameters

- rate - int

Results

- previousRate - int
`,
	})
}

// Terminates app
func rcSetMutexProfileFraction(ctx context.Context, in Params) (out Params, err error) {
	rate, err := in.GetInt64("rate")
	if err != nil {
		return nil, err
	}
	previousRate := runtime.SetMutexProfileFraction(int(rate))
	out = make(Params)
	out["previousRate"] = previousRate
	return out, nil
}

func init() {
	Add(Call{
		Path:  "debug/set-block-profile-rate",
		Fn:    rcSetBlockProfileRate,
		Title: "Set runtime.SetBlockProfileRate for blocking profiling.",
		Help: `
SetBlockProfileRate controls the fraction of goroutine blocking events
that are reported in the blocking profile. The profiler aims to sample
an average of one blocking event per rate nanoseconds spent blocked.

To include every blocking event in the profile, pass rate = 1. To turn
off profiling entirely, pass rate <= 0.

After calling this you can use this to see the blocking profile:

    go tool pprof http://localhost:5572/debug/pprof/block

Parameters

- rate - int
`,
	})
}

// Terminates app
func rcSetBlockProfileRate(ctx context.Context, in Params) (out Params, err error) {
	rate, err := in.GetInt64("rate")
	if err != nil {
		return nil, err
	}
	runtime.SetBlockProfileRate(int(rate))
	return nil, nil
}

func init() {
	Add(Call{
		Path:          "core/command",
		AuthRequired:  true,
		Fn:            rcRunCommand,
		NeedsRequest:  true,
		NeedsResponse: true,
		Title:         "Run a rclone terminal command over rc.",
		Help: `This takes the following parameters

- command - a string with the command name
- arg - a list of arguments for the backend command
- opt - a map of string to string of options
- returnType - one of ("COMBINED_OUTPUT", "STREAM", "STREAM_ONLY_STDOUT", "STREAM_ONLY_STDERR")
    - defaults to "COMBINED_OUTPUT" if not set
    - the STREAM returnTypes will write the output to the body of the HTTP message
    - the COMBINED_OUTPUT will write the output to the "result" parameter

Returns

- result - result from the backend command
    - only set when using returnType "COMBINED_OUTPUT"
- error	 - set if rclone exits with an error code
- returnType - one of ("COMBINED_OUTPUT", "STREAM", "STREAM_ONLY_STDOUT", "STREAM_ONLY_STDERR")

For example

    rclone rc core/command command=ls -a mydrive:/ -o max-depth=1
    rclone rc core/command -a ls -a mydrive:/ -o max-depth=1

Returns

` + "```" + `
{
	"error": false,
	"result": "<Raw command line output>"
}

OR 
{
	"error": true,
	"result": "<Raw command line output>"
}

` + "```" + `
`,
	})
}

// rcRunCommand runs an rclone command with the given args and flags
func rcRunCommand(ctx context.Context, in Params) (out Params, err error) {
	command, err := in.GetString("command")
	if err != nil {
		command = ""
	}

	var opt = map[string]string{}
	err = in.GetStructMissingOK("opt", &opt)
	if err != nil {
		return nil, err
	}

	var arg = []string{}
	err = in.GetStructMissingOK("arg", &arg)
	if err != nil {
		return nil, err
	}

	returnType, err := in.GetString("returnType")
	if err != nil {
		returnType = "COMBINED_OUTPUT"
	}

	var httpResponse http.ResponseWriter
	httpResponse, err = in.GetHTTPResponseWriter()
	if err != nil {
		return nil, errors.Errorf("response object is required\n" + err.Error())
	}

	var allArgs = []string{}
	if command != "" {
		// Add the command e.g.: ls to the args
		allArgs = append(allArgs, command)
	}
	// Add all from arg
	for _, cur := range arg {
		allArgs = append(allArgs, cur)
	}

	// Add flags to args for e.g. --max-depth 1 comes in as { max-depth 1 }.
	// Convert it to [ max-depth, 1 ] and append to args list
	for key, value := range opt {
		if len(key) == 1 {
			allArgs = append(allArgs, "-"+key)
		} else {
			allArgs = append(allArgs, "--"+key)
		}
		allArgs = append(allArgs, value)
	}

	// Get the path for the current executable which was used to run rclone.
	ex, err := os.Executable()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, ex, allArgs...)

	if returnType == "COMBINED_OUTPUT" {
		// Run the command and get the output for error and stdout combined.

		out, err := cmd.CombinedOutput()

		if err != nil {
			return Params{
				"result": string(out),
				"error":  true,
			}, nil
		}
		return Params{
			"result": string(out),
			"error":  false,
		}, nil
	} else if returnType == "STREAM_ONLY_STDOUT" {
		cmd.Stdout = httpResponse
	} else if returnType == "STREAM_ONLY_STDERR" {
		cmd.Stderr = httpResponse
	} else if returnType == "STREAM" {
		cmd.Stdout = httpResponse
		cmd.Stderr = httpResponse
	} else {
		return nil, errors.Errorf("Unknown returnType %q", returnType)
	}

	err = cmd.Run()
	return nil, err
}
