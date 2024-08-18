//go:build !plan9
// +build !plan9

package fserrors

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
)

// make a plausible network error with the underlying errno
func makeNetErr(errno syscall.Errno) error {
	return &net.OpError{
		Op:     "write",
		Net:    "tcp",
		Source: &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 123},
		Addr:   &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 8080},
		Err: &os.SyscallError{
			Syscall: "write",
			Err:     errno,
		},
	}
}

func TestWithSyscallCause(t *testing.T) {
	for i, test := range []struct {
		err           error
		wantRetriable bool
		wantErr       error
	}{
		{makeNetErr(syscall.EAGAIN), true, syscall.EAGAIN},
		{makeNetErr(syscall.Errno(123123123)), false, syscall.Errno(123123123)},
	} {
		gotRetriable, gotErr := Cause(test.err)
		what := fmt.Sprintf("test #%d: %v", i, test.err)
		assert.Equal(t, test.wantErr, gotErr, what)
		assert.Equal(t, test.wantRetriable, gotRetriable, what)
	}
}

func TestWithSyscallShouldRetry(t *testing.T) {
	for i, test := range []struct {
		err  error
		want bool
	}{
		{makeNetErr(syscall.EAGAIN), true},
		{makeNetErr(syscall.Errno(123123123)), false},
		{
			wrap(&url.Error{
				Op:  "post",
				URL: "http://localhost/",
				Err: makeNetErr(syscall.EPIPE),
			}, "potato error"),
			true,
		},
		{
			wrap(&url.Error{
				Op:  "post",
				URL: "http://localhost/",
				Err: makeNetErr(syscall.Errno(123123123)),
			}, "listing error"),
			false,
		},
	} {
		got := ShouldRetry(test.err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test #%d: %v", i, test.err))
	}
}
