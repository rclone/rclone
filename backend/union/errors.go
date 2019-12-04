package union

import (
	"bytes"
	"fmt"
)

// The Errors type wraps a slice of errors
type Errors []error

var (
	// FilterNil returns the error directly
	FilterNil = func(err error) error {
		return err
	}
)

// Map returns a copy of the error slice with all its errors modified
// according to the mapping function. If mapping returns nil,
// the error is dropped from the error slice with no replacement.
func (e Errors) Map(mapping func(error) error) Errors {
	s := make([]error, len(e))
	i := 0
	for _, err := range e {
		nerr := mapping(err)
		if nerr == nil {
			continue
		}
		s[i] = nerr
		i++
	}
	return Errors(s[:i])
}

// Err returns a MultiError struct containing this Errors instance, or nil
// if there are zero errors contained.
func (e Errors) Err() error {
	e = e.Map(FilterNil)
	if len(e) == 0 {
		return nil
	}

	return &MultiError{Errors: e}
}

// MultiError type implements the error interface, and contains the
// Errors used to construct it.
type MultiError struct {
	Errors Errors
}

// Error returns a concatenated string of the contained errors
func (m *MultiError) Error() string {
	var buf bytes.Buffer

	if len(m.Errors) == 1 {
		buf.WriteString("1 error: ")
	} else {
		fmt.Fprintf(&buf, "%d errors: ", len(m.Errors))
	}

	for i, err := range m.Errors {
		if i != 0 {
			buf.WriteString("; ")
		}

		buf.WriteString(err.Error())
	}

	return buf.String()
}
