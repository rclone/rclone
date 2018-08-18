package fs

// RemoteList is parsed by flag with k/M/G suffixes
import (
	"fmt"
	"github.com/pkg/errors"
	"strings"
)

// RemoteList is an []string with a friendly way of printing setting
type RemoteList []string

// String turns RemoteList into a string
func (x RemoteList) String() string {
	return strings.Join(x, ",")
}

// Set a RemoteList
func (x *RemoteList) Set(s string) error {
	if len(s) == 0 {
		return errors.New("empty string")
	}
	*x = strings.Split(s, ",")
	return nil
}

// Type of the value
func (x *RemoteList) Type() string {
	return "[]string"
}

// Scan implements the fmt.Scanner interface
func (x *RemoteList) Scan(s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, nil)
	if err != nil {
		return err
	}
	return x.Set(string(token))
}
