// Copyright (C) 2014 Space Monkey, Inc.
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

package errors

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

var (
	logOnCreation      = GenSym()
	captureStack       = GenSym()
	disableInheritance = GenSym()
)

// ErrorClass is the basic hierarchical error type. An ErrorClass generates
// actual errors, but the error class controls properties of the errors it
// generates, such as where those errors are in the hierarchy, whether or not
// they capture the stack on instantiation, and so forth.
type ErrorClass struct {
	parent *ErrorClass
	name   string
	data   map[DataKey]interface{}
}

var (
	// HierarchicalError is the base class for all hierarchical errors generated
	// through this class.
	HierarchicalError = &ErrorClass{
		parent: nil,
		name:   "Error",
		data:   map[DataKey]interface{}{captureStack: true}}

	// SystemError is the base error class for errors not generated through this
	// errors library. It is not expected that anyone would ever generate new
	// errors from a SystemError type or make subclasses.
	SystemError = &ErrorClass{
		parent: nil,
		name:   "System Error",
		data:   map[DataKey]interface{}{}}
)

// An ErrorOption is something that controls behavior of specific error
// instances. They can be set on ErrorClasses or errors individually.
type ErrorOption func(map[DataKey]interface{})

// SetData will take the given value and store it with the error or error class
// and its descendents associated with the given DataKey. Be sure to check out
// the example. value can be nil to disable values for subhierarchies.
func SetData(key DataKey, value interface{}) ErrorOption {
	return func(m map[DataKey]interface{}) {
		m[key] = value
	}
}

// LogOnCreation tells the error class and its descendents to log the stack
// whenever an error of this class is created.
func LogOnCreation() ErrorOption {
	return SetData(logOnCreation, true)
}

// CaptureStack tells the error class and its descendents to capture the stack
// whenever an error of this class is created, and output it as part of the
// error's Error() method. This is the default.
func CaptureStack() ErrorOption {
	return SetData(captureStack, true)
}

// NoLogOnCreation is the opposite of LogOnCreation and applies to the error,
// class, and its descendents. This is the default.
func NoLogOnCreation() ErrorOption {
	return SetData(logOnCreation, false)
}

// NoCaptureStack is the opposite of CaptureStack and applies to the error,
// class, and its descendents.
func NoCaptureStack() ErrorOption {
	return SetData(captureStack, false)
}

// If DisableInheritance is provided, the error or error class will belong to
// its ancestors, but will not inherit their settings and options. Use with
// caution, and may disappear in future releases.
func DisableInheritance() ErrorOption {
	return SetData(disableInheritance, true)
}

func boolWrapper(val interface{}, default_value bool) bool {
	rv, ok := val.(bool)
	if ok {
		return rv
	}
	return default_value
}

// NewClass creates an error class with the provided name and options. Classes
// generated from this method and not *ErrorClass.NewClass will descend from
// the root HierarchicalError base class.
func NewClass(name string, options ...ErrorOption) *ErrorClass {
	return HierarchicalError.NewClass(name, options...)
}

// New is for compatibility with the default Go errors package. It simply
// creates an error from the HierarchicalError root class.
func New(text string) error {
	// NewWith doesn't take a format string, even though we have no options.
	return HierarchicalError.NewWith(text)
}

// NewClass creates an error class with the provided name and options. The new
// class will descend from the receiver.
func (parent *ErrorClass) NewClass(name string,
	options ...ErrorOption) *ErrorClass {

	ec := &ErrorClass{
		parent: parent,
		name:   name,
		data:   make(map[DataKey]interface{})}

	for _, option := range options {
		option(ec.data)
	}

	if !boolWrapper(ec.data[disableInheritance], false) {
		// hoist options for speed
		for key, val := range parent.data {
			_, exists := ec.data[key]
			if !exists {
				ec.data[key] = val
			}
		}
		return ec
	} else {
		delete(ec.data, disableInheritance)
	}

	return ec
}

// MustAddData allows adding data key value pairs to error classes after they
// are created. This is useful for allowing external packages add namespaced
// values to errors defined outside of their package. It will panic if the
// key is already set in the error class.
func (e *ErrorClass) MustAddData(key DataKey, value interface{}) {
	if _, ex := e.data[key]; ex {
		panic("key already exists")
	}
	e.data[key] = value
}

// GetData will return any data set on the error class for the given key. It
// returns nil if there is no data set for that key.
func (e *ErrorClass) GetData(key DataKey) interface{} {
	return e.data[key]
}

// Parent returns this error class' direct ancestor.
func (e *ErrorClass) Parent() *ErrorClass {
	return e.parent
}

// String returns this error class' name
func (e *ErrorClass) String() string {
	if e == nil {
		return "nil"
	}
	return e.name
}

// Is returns true if the receiver class is or is a descendent of parent.
func (e *ErrorClass) Is(parent *ErrorClass) bool {
	for check := e; check != nil; check = check.parent {
		if check == parent {
			return true
		}
	}
	return false
}

// frame logs the pc at some point during execution.
type frame struct {
	pc uintptr
}

// String returns a human readable form of the frame.
func (e frame) String() string {
	if e.pc == 0 {
		return "unknown.unknown:0"
	}
	f := runtime.FuncForPC(e.pc)
	if f == nil {
		return "unknown.unknown:0"
	}
	file, line := f.FileLine(e.pc)
	return fmt.Sprintf("%s:%s:%d", f.Name(), filepath.Base(file), line)
}

// callerState records the pc into an frame for two callers up.
func callerState(depth int) frame {
	pc, _, _, ok := runtime.Caller(depth)
	if !ok {
		return frame{pc: 0}
	}
	return frame{pc: pc}
}

// record will record the pc at the given depth into the error if it is
// capable of recording it.
func record(err error, depth int) error {
	if err == nil {
		return nil
	}
	cast, ok := err.(*Error)
	if !ok {
		return err
	}
	cast.exits = append(cast.exits, callerState(depth))
	return cast
}

// Record will record the current pc on the given error if possible, adding
// to the error's recorded exits list. Returns the given error argument.
func Record(err error) error {
	return record(err, 3)
}

// RecordBefore will record the pc depth frames above the current stack frame
// on the given error if possible, adding to the error's recorded exits list.
// Record(err) is equivalent to RecordBefore(err, 0). Returns the given error
// argument.
func RecordBefore(err error, depth int) error {
	return record(err, 3+depth)
}

// Error is the type that represents a specific error instance. It is not
// expected that you will work with *Error classes directly. Instead, you
// should use the 'error' interface and errors package methods that operate
// on errors instances.
type Error struct {
	err    error
	class  *ErrorClass
	stacks [][]frame
	exits  []frame
	data   map[DataKey]interface{}
}

// GetData returns the value associated with the given DataKey on this error
// or any of its ancestors. Please see the example for SetData
func (e *Error) GetData(key DataKey) interface{} {
	if e.data != nil {
		val, ok := e.data[key]
		if ok {
			return val
		}
		if boolWrapper(e.data[disableInheritance], false) {
			return nil
		}
	}
	return e.class.data[key]
}

// GetData returns the value associated with the given DataKey on this error
// or any of its ancestors. Please see the example for SetData
func GetData(err error, key DataKey) interface{} {
	cast, ok := err.(*Error)
	if ok {
		return cast.GetData(key)
	}
	return nil
}

func (e *ErrorClass) wrap(err error, classes []*ErrorClass,
	options []ErrorOption) error {
	if err == nil {
		return nil
	}
	if ec, ok := err.(*Error); ok {
		if ec.Is(e) {
			if len(options) == 0 {
				return ec
			}
			// if we have options, we have to wrap it cause we don't want to
			// mutate the existing error.
		} else {
			for _, class := range classes {
				if ec.Is(class) {
					return err
				}
			}
		}
	}

	rv := &Error{err: err, class: e}
	if len(options) > 0 {
		rv.data = make(map[DataKey]interface{})
		for _, option := range options {
			option(rv.data)
		}
	}

	if boolWrapper(rv.GetData(captureStack), false) {
		rv.stacks = [][]frame{getStack(3)}
	}
	if boolWrapper(rv.GetData(logOnCreation), false) {
		LogWithStack(rv.Error())
	}
	return rv
}

func getStack(depth int) (stack []frame) {
	var pcs [256]uintptr
	amount := runtime.Callers(depth+1, pcs[:])
	stack = make([]frame, amount)
	for i := 0; i < amount; i++ {
		stack[i] = frame{pcs[i]}
	}
	return stack
}

// AttachStack adds another stack to the current error's stack trace if it
// exists
func AttachStack(err error) {
	if err == nil {
		return
	}
	cast, ok := err.(*Error)
	if !ok {
		return
	}
	if len(cast.stacks) < 1 {
		// only record stacks if this error was supposed to
		return
	}
	cast.stacks = append(cast.stacks, getStack(2))
}

// WrapUnless wraps the given error in the receiver error class unless the
// error is already an instance of one of the provided error classes.
func (e *ErrorClass) WrapUnless(err error, classes ...*ErrorClass) error {
	return e.wrap(err, classes, nil)
}

// Wrap wraps the given error in the receiver error class with the provided
// error-specific options.
func (e *ErrorClass) Wrap(err error, options ...ErrorOption) error {
	return e.wrap(err, nil, options)
}

// New makes a new error type. It takes a format string.
func (e *ErrorClass) New(format string, args ...interface{}) error {
	return e.wrap(fmt.Errorf(format, args...), nil, nil)
}

// NewWith makes a new error type with the provided error-specific options.
func (e *ErrorClass) NewWith(message string, options ...ErrorOption) error {
	return e.wrap(errors.New(message), nil, options)
}

// Error conforms to the error interface. Error will return the backtrace if
// it was captured and any recorded exits.
func (e *Error) Error() string {
	message := strings.TrimRight(e.err.Error(), "\n ")
	if strings.Contains(message, "\n") {
		message = fmt.Sprintf("%s:\n  %s", e.class.String(),
			strings.Replace(message, "\n", "\n  ", -1))
	} else {
		message = fmt.Sprintf("%s: %s", e.class.String(), message)
	}
	if stack := e.Stack(); stack != "" {
		message = fmt.Sprintf(
			"%s\n\"%s\" backtrace:\n%s", message, e.class, stack)
	}
	if exits := e.Exits(); exits != "" {
		message = fmt.Sprintf(
			"%s\n\"%s\" exits:\n%s", message, e.class, exits)
	}
	return message
}

// Message returns just the error message without the backtrace or exits.
func (e *Error) Message() string {
	message := strings.TrimRight(GetMessage(e.err), "\n ")
	if strings.Contains(message, "\n") {
		return fmt.Sprintf("%s:\n  %s", e.class.String(),
			strings.Replace(message, "\n", "\n  ", -1))
	}
	return fmt.Sprintf("%s: %s", e.class.String(), message)
}

// WrappedErr returns the wrapped error, if the current error is simply
// wrapping some previously returned error or system error. You probably want
// the package-level WrappedErr
func (e *Error) WrappedErr() error {
	return e.err
}

// WrappedErr returns the wrapped error, if the current error is simply
// wrapping some previously returned error or system error. If the error isn't
// hierarchical it is just returned.
func WrappedErr(err error) error {
	cast, ok := err.(*Error)
	if !ok {
		return err
	}
	return cast.WrappedErr()
}

// Class will return the appropriate error class for the given error. You
// probably want the package-level GetClass.
func (e *Error) Class() *ErrorClass {
	return e.class
}

// Name returns the name of the error: in this case the name of the class the
// error belongs to.
func (e *Error) Name() (string, bool) {
	return e.class.name, true
}

// GetClass will return the appropriate error class for the given error.
// If the error is not nil, GetClass always returns a hierarchical error class,
// and even attempts to determine a class for common system error types.
func GetClass(err error) *ErrorClass {
	if err == nil {
		return nil
	}
	cast, ok := err.(*Error)
	if !ok {
		return findSystemErrorClass(err)
	}
	return cast.class
}

// Stack will return the stack associated with the error if one is found. You
// probably want the package-level GetStack.
func (e *Error) Stack() string {
	if len(e.stacks) > 0 {
		var frames []string
		for _, stack := range e.stacks {
			if frames == nil {
				frames = make([]string, 0, len(stack))
			} else {
				frames = append(frames, "----- attached stack -----")
			}
			for _, f := range stack {
				frames = append(frames, f.String())
			}
		}
		return strings.Join(frames, "\n")
	}
	return ""
}

// GetStack will return the stack associated with the error if one is found.
func GetStack(err error) string {
	if err == nil {
		return ""
	}
	cast, ok := err.(*Error)
	if !ok {
		return ""
	}
	return cast.Stack()
}

// Exits will return the exits recorded on the error if any are found. You
// probably want the package-level GetExits.
func (e *Error) Exits() string {
	if len(e.exits) > 0 {
		exits := make([]string, len(e.exits))
		for i, ex := range e.exits {
			exits[i] = ex.String()
		}
		return strings.Join(exits, "\n")
	}
	return ""
}

// GetExits will return the exits recorded on the error if any are found.
func GetExits(err error) string {
	if err == nil {
		return ""
	}
	cast, ok := err.(*Error)
	if !ok {
		return ""
	}
	return cast.Exits()
}

// GetMessage returns just the error message without the backtrace or exits.
func GetMessage(err error) string {
	if err == nil {
		return ""
	}
	cast, ok := err.(*Error)
	if !ok {
		return err.Error()
	}
	return cast.Message()
}

// EquivalenceOption values control behavior of determining whether or not an
// error belongs to a specific class.
type EquivalenceOption int

const (
	// If IncludeWrapped is used, wrapped errors are also used for determining
	// class membership.
	IncludeWrapped EquivalenceOption = 1
)

func combineEquivOpts(opts []EquivalenceOption) (rv EquivalenceOption) {
	for _, opt := range opts {
		rv |= opt
	}
	return rv
}

// Is returns whether or not an error belongs to a specific class. Typically
// you should use Contains instead.
func (e *Error) Is(ec *ErrorClass, opts ...EquivalenceOption) bool {
	return ec.Contains(e, opts...)
}

// Contains returns whether or not the receiver error class contains the given
// error instance.
func (e *ErrorClass) Contains(err error, opts ...EquivalenceOption) bool {
	if err == nil {
		return false
	}
	cast, ok := err.(*Error)
	if !ok {
		return findSystemErrorClass(err).Is(e)
	}
	if cast.class.Is(e) {
		return true
	}
	if combineEquivOpts(opts)&IncludeWrapped == 0 {
		return false
	}
	return e.Contains(cast.err, opts...)
}

var (
	// Useful error classes
	NotImplementedError = NewClass("Not Implemented Error", LogOnCreation())
	ProgrammerError     = NewClass("Programmer Error", LogOnCreation())
	PanicError          = NewClass("Panic Error", LogOnCreation())

	// The following SystemError descendants are provided such that the GetClass
	// method has something to return for standard library error types not
	// defined through this class.
	//
	// It is not expected that anyone would create instances of these classes.
	//
	// from os
	SyscallError = SystemError.NewClass("Syscall Error")
	// from syscall
	ErrnoError = SystemError.NewClass("Errno Error")
	// from net
	NetworkError        = SystemError.NewClass("Network Error")
	UnknownNetworkError = NetworkError.NewClass("Unknown Network Error")
	AddrError           = NetworkError.NewClass("Addr Error")
	InvalidAddrError    = AddrError.NewClass("Invalid Addr Error")
	NetOpError          = NetworkError.NewClass("Network Op Error")
	NetParseError       = NetworkError.NewClass("Network Parse Error")
	DNSError            = NetworkError.NewClass("DNS Error")
	DNSConfigError      = DNSError.NewClass("DNS Config Error")
	// from io
	IOError            = SystemError.NewClass("IO Error")
	EOF                = IOError.NewClass("EOF")
	ClosedPipeError    = IOError.NewClass("Closed Pipe Error")
	NoProgressError    = IOError.NewClass("No Progress Error")
	ShortBufferError   = IOError.NewClass("Short Buffer Error")
	ShortWriteError    = IOError.NewClass("Short Write Error")
	UnexpectedEOFError = IOError.NewClass("Unexpected EOF Error")
	// from context
	ContextError    = SystemError.NewClass("Context Error")
	ContextCanceled = ContextError.NewClass("Canceled")
	ContextTimeout  = ContextError.NewClass("Timeout")
)

func findSystemErrorClass(err error) *ErrorClass {
	switch err {
	case io.EOF:
		return EOF
	case io.ErrUnexpectedEOF:
		return UnexpectedEOFError
	case io.ErrClosedPipe:
		return ClosedPipeError
	case io.ErrNoProgress:
		return NoProgressError
	case io.ErrShortBuffer:
		return ShortBufferError
	case io.ErrShortWrite:
		return ShortWriteError
	case contextCanceled:
		return ContextCanceled
	case contextDeadlineExceeded:
		return ContextTimeout
	default:
		break
	}
	if isErrnoError(err) {
		return ErrnoError
	}
	switch err.(type) {
	case *os.SyscallError:
		return SyscallError
	case net.UnknownNetworkError:
		return UnknownNetworkError
	case *net.AddrError:
		return AddrError
	case net.InvalidAddrError:
		return InvalidAddrError
	case *net.OpError:
		return NetOpError
	case *net.ParseError:
		return NetParseError
	case *net.DNSError:
		return DNSError
	case *net.DNSConfigError:
		return DNSConfigError
	case net.Error:
		return NetworkError
	default:
		return SystemError
	}
}
