// Define the internal rc functions

package rc

import (
	"os"

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
		Path:  "rc/pid",
		Fn:    rcPid,
		Title: "Return PID of current process",
		Help: `
This returns PID of current process.
Useful for stopping rclone process.`,
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
