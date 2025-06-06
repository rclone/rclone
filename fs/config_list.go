package fs

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// CommaSepList is a comma separated config value
// It uses the encoding/csv rules for quoting and escaping
type CommaSepList []string

// SpaceSepList is a space separated config value
// It uses the encoding/csv rules for quoting and escaping
type SpaceSepList []string

type genericList []string

func (l CommaSepList) String() string {
	return genericList(l).string(',')
}

// Set the List entries
func (l *CommaSepList) Set(s string) error {
	return (*genericList)(l).set(',', []byte(s))
}

// Type of the value
func (CommaSepList) Type() string {
	return "CommaSepList"
}

// Scan implements the fmt.Scanner interface
func (l *CommaSepList) Scan(s fmt.ScanState, ch rune) error {
	return (*genericList)(l).scan(',', s, ch)
}

func (l SpaceSepList) String() string {
	return genericList(l).string(' ')
}

// Set the List entries
func (l *SpaceSepList) Set(s string) error {
	return (*genericList)(l).set(' ', []byte(s))
}

// Type of the value
func (SpaceSepList) Type() string {
	return "SpaceSepList"
}

// Scan implements the fmt.Scanner interface
func (l *SpaceSepList) Scan(s fmt.ScanState, ch rune) error {
	return (*genericList)(l).scan(' ', s, ch)
}

func (gl genericList) string(sep rune) string {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	w.Comma = sep
	err := w.Write(gl)
	if err != nil {
		// can only happen if w.Comma is invalid
		panic(err)
	}
	w.Flush()
	return string(bytes.TrimSpace(buf.Bytes()))
}

func (gl *genericList) set(sep rune, b []byte) error {
	if len(b) == 0 {
		*gl = nil
		return nil
	}
	r := csv.NewReader(bytes.NewReader(b))
	r.Comma = sep

	record, err := r.Read()
	switch _err := err.(type) {
	case nil:
		*gl = record
	case *csv.ParseError:
		err = _err.Err // remove line numbers from the error message
	}
	return err
}

func (gl *genericList) scan(sep rune, s fmt.ScanState, ch rune) error {
	token, err := s.Token(true, func(rune) bool { return true })
	if err != nil {
		return err
	}
	return gl.set(sep, bytes.TrimSpace(token))
}

// ExecCommand executes a command and returns the output as a string
// It returns an error if the command fails or the output is empty
func ExecCommand(l SpaceSepList) (pass string, err error) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(l[0], l[1:]...)

	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	cmd.Stdin = os.Stdin

	err = cmd.Run()
	if err != nil {
		// One does not always get the stderr returned in the wrapped error.
		Errorf(nil, "Executing command %q failed: %v", l, err)
		if ers := strings.TrimSpace(stderr.String()); ers != "" {
			Errorf(nil, "stderr: %q", ers)
		}
		return pass, fmt.Errorf("executing command %q failed: %w", l, err)
	}
	pass = strings.Trim(stdout.String(), "\r\n")
	if pass == "" {
		return pass, errors.New("executing command %q failed: returned empty string")
	}
	return pass, nil

}
