// Define the internal rc functions

package rc

import (
	"os"
	"runtime"

	"github.com/pkg/errors"
)

func init() {
	Add(Call{
		Path:  "rc/noop",
		Fn:    rcNoop,
		Title: "Echo the input to the output parameters",
		Help: `
This echoes the input parameters to the output parameters for testing
purposes.  It can be used to check that rclone is still alive and to
check that parameter passing is working properly.`,
	})
	Add(Call{
		Path:  "rc/error",
		Fn:    rcError,
		Title: "This returns an error",
		Help: `
This returns an error with the input as part of its error string.
Useful for testing error handling.`,
	})
	Add(Call{
		Path:  "rc/list",
		Fn:    rcList,
		Title: "List all the registered remote control commands",
		Help: `
This lists all the registered remote control commands as a JSON map in
the commands response.`,
	})
	Add(Call{
		Path:  "core/pid",
		Fn:    rcPid,
		Title: "Return PID of current process",
		Help: `
This returns PID of current process.
Useful for stopping rclone process.`,
	})
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

// Echo the input to the ouput parameters
func rcNoop(in Params) (out Params, err error) {
	return in, nil
}

// Return an error regardless
func rcError(in Params) (out Params, err error) {
	return nil, errors.Errorf("arbitrary error on input %+v", in)
}

// List the registered commands
func rcList(in Params) (out Params, err error) {
	out = make(Params)
	out["commands"] = registry.list()
	return out, nil
}

// Return PID of current process
func rcPid(in Params) (out Params, err error) {
	out = make(Params)
	out["pid"] = os.Getpid()
	return out, nil
}

// Return the memory statistics
func rcMemStats(in Params) (out Params, err error) {
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

// Do a garbage collection run
func rcGc(in Params) (out Params, err error) {
	out = make(Params)
	runtime.GC()
	return out, nil
}
