// Copyright (C) 2017 Space Monkey, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package monkit

import (
	"context"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
)

// errorNameHandlers keeps track of the list of error name handlers monkit will
// call to give errors good metric names.
var errorNameHandlers struct {
	write_mu sync.Mutex
	value    atomic.Value
}

// AddErrorNameHandler adds an error name handler function that will be
// consulted every time an error is captured for a task. The handlers will be
// called in the order they were registered with the most recently added
// handler first, until a handler returns true for the second return value.
// If no handler returns true, the error is checked to see if it implements
// an interface that allows it to name itself, and otherwise, monkit attempts
// to find a good name for most built in Go standard library errors.
func AddErrorNameHandler(f func(error) (string, bool)) {
	errorNameHandlers.write_mu.Lock()
	defer errorNameHandlers.write_mu.Unlock()

	handlers, _ := errorNameHandlers.value.Load().([]func(error) (string, bool))
	handlers = append(handlers, f)
	errorNameHandlers.value.Store(handlers)
}

// getErrorName implements the logic described in the AddErrorNameHandler
// function.
func getErrorName(err error) string {
	// check if any of the handlers will handle it
	handlers, _ := errorNameHandlers.value.Load().([]func(error) (string, bool))
	for i := len(handlers) - 1; i >= 0; i-- {
		if name, ok := handlers[i](err); ok {
			return name
		}
	}

	// check if it knows how to name itself
	type namer interface {
		Name() (string, bool)
	}

	if n, ok := err.(namer); ok {
		if name, ok := n.Name(); ok {
			return name
		}
	}

	// check if it's a known error that we handle to give good names
	switch err {
	case io.EOF:
		return "EOF"
	case io.ErrUnexpectedEOF:
		return "Unexpected EOF Error"
	case io.ErrClosedPipe:
		return "Closed Pipe Error"
	case io.ErrNoProgress:
		return "No Progress Error"
	case io.ErrShortBuffer:
		return "Short Buffer Error"
	case io.ErrShortWrite:
		return "Short Write Error"
	case context.Canceled:
		return "Canceled"
	case context.DeadlineExceeded:
		return "Timeout"
	}
	if isErrnoError(err) {
		return "Errno"
	}
	switch err.(type) {
	case *os.SyscallError:
		return "Syscall Error"
	case net.UnknownNetworkError:
		return "Unknown Network Error"
	case *net.AddrError:
		return "Addr Error"
	case net.InvalidAddrError:
		return "Invalid Addr Error"
	case *net.OpError:
		return "Net Op Error"
	case *net.ParseError:
		return "Net Parse Error"
	case *net.DNSError:
		return "DNS Error"
	case *net.DNSConfigError:
		return "DNS Config Error"
	case net.Error:
		return "Network Error"
	}
	return "System Error"
}
