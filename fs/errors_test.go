package fs

import (
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"syscall"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

var errUseOfClosedNetworkConnection = errors.New("use of closed network connection")

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

func TestIsClosedConnError(t *testing.T) {
	for i, test := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("potato"), false},
		{errUseOfClosedNetworkConnection, true},
		{makeNetErr(syscall.EAGAIN), true},
		{makeNetErr(syscall.Errno(123123123)), false},
	} {
		got := isClosedConnError(test.err)
		assert.Equal(t, test.want, got, fmt.Sprintf("test #%d: %v", i, test.err))
	}
}

func TestShouldRetry(t *testing.T) {
	for i, test := range []struct {
		err  error
		want bool
	}{
		{nil, false},
		{errors.New("potato"), false},
		{errors.Wrap(errUseOfClosedNetworkConnection, "connection"), true},
		{io.EOF, true},
		{io.ErrUnexpectedEOF, true},
		{&url.Error{Op: "post", URL: "/", Err: io.EOF}, true},
		{&url.Error{Op: "post", URL: "/", Err: errUseOfClosedNetworkConnection}, true},
		{
			errors.Wrap(&url.Error{
				Op:  "post",
				URL: "http://localhost/",
				Err: makeNetErr(syscall.EPIPE),
			}, "potato error"),
			true,
		},
		{
			errors.Wrap(&url.Error{
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
