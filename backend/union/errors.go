package union

import (
	"bytes"
	"fmt"
)

// The Errors type wraps a slice of errors
type Errors []error

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

// FilterNil returns the Errors without nil
func (e Errors) FilterNil() Errors {
	ne := e.Map(func(err error) error {
		return err
	})
	return ne
}

// Err returns an error interface that filtered nil,
// or nil if no non-nil Error is presented.
func (e Errors) Err() error {
	ne := e.FilterNil()
	if len(ne) == 0 {
		return nil
	}
	return ne
}

// Error returns a concatenated string of the contained errors
func (e Errors) Error() string {
	var buf bytes.Buffer

	if len(e) == 0 {
		buf.WriteString("no error")
	} else if len(e) == 1 {
		buf.WriteString("1 error: ")
	} else {
		fmt.Fprintf(&buf, "%d errors: ", len(e))
	}

	for i, err := range e {
		if i != 0 {
			buf.WriteString("; ")
		}

		if err != nil {
			buf.WriteString(err.Error())
		} else {
			buf.WriteString("nil error")
		}
	}

	return buf.String()
}

// Unwrap returns the wrapped errors
func (e Errors) Unwrap() []error {
	return e
}
