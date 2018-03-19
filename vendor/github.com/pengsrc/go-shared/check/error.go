package check

import (
	"fmt"
	"os"
)

// ReadableError is just a structure contains readable message that can be
// returned directly to end user.
type ReadableError struct {
	Message string
}

// Error returns the description of ReadableError.
func (e ReadableError) Error() string {
	return e.Message
}

// NewReadableError creates a ReadableError{} from given message.
func NewReadableError(message string) ReadableError {
	return ReadableError{Message: message}
}

// ErrorForExit check the error.
// If error is not nil, print the error message and exit the application.
// If error is nil, do nothing.
func ErrorForExit(name string, err error, code ...int) {
	if err != nil {
		exitCode := 1
		if len(code) > 0 {
			exitCode = code[0]
		}
		fmt.Fprintf(os.Stderr, "%s: %s (%d)\n", name, err.Error(), exitCode)
		fmt.Fprintf(os.Stderr, "See \"%s --help\".\n", name)
		os.Exit(exitCode)
	}
}
