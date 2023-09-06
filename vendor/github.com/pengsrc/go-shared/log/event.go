package log

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pengsrc/go-shared/buffer"
	"github.com/pengsrc/go-shared/convert"
)

// A EventCallerPool is a type-safe wrapper around a sync.BytesBufferPool.
type EventCallerPool struct {
	p *sync.Pool
}

// NewEventCallerPool constructs a new BytesBufferPool.
func NewEventCallerPool() EventCallerPool {
	return EventCallerPool{
		p: &sync.Pool{
			New: func() interface{} {
				return &EventCaller{}
			},
		},
	}
}

// Get retrieves a EventCaller from the pool, creating one if necessary.
func (p EventCallerPool) Get() *EventCaller {
	e := p.p.Get().(*EventCaller)

	e.pool = p

	e.Defined = false
	e.PC = 0
	e.File = ""
	e.Line = 0

	return e
}

func (p EventCallerPool) put(caller *EventCaller) {
	p.p.Put(caller)
}

// EventCaller represents the caller of a logging function.
type EventCaller struct {
	pool EventCallerPool

	Defined bool
	PC      uintptr
	File    string
	Line    int
}

// Free returns the EventCaller to its EventCallerPool.
// Callers must not retain references to the EventCaller after calling Free.
func (ec *EventCaller) Free() {
	ec.pool.put(ec)
}

// String returns the full path and line number of the caller.
func (ec EventCaller) String() string {
	return ec.FullPath()
}

// FullPath returns a /full/path/to/package/file:line description of the
// caller.
func (ec EventCaller) FullPath() string {
	if !ec.Defined {
		return "undefined"
	}
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()
	buf.AppendString(ec.File)
	buf.AppendByte(':')
	buf.AppendInt(int64(ec.Line))
	return buf.String()
}

// TrimmedPath returns a package/file:line description of the caller,
// preserving only the leaf directory name and file name.
func (ec EventCaller) TrimmedPath() string {
	if !ec.Defined {
		return "undefined"
	}
	// nb. To make sure we trim the path correctly on Windows too, we
	// counter-intuitively need to use '/' and *not* os.PathSeparator here,
	// because the path given originates from Go stdlib, specifically
	// runtime.Caller() which (as of Mar/17) returns forward slashes even on
	// Windows.
	//
	// See https://github.com/golang/go/issues/3335
	// and https://github.com/golang/go/issues/18151
	//
	// for discussion on the issue on Go side.
	//
	// Find the last separator.
	//
	idx := strings.LastIndexByte(ec.File, '/')
	if idx == -1 {
		return ec.FullPath()
	}
	// Find the penultimate separator.
	idx = strings.LastIndexByte(ec.File[:idx], '/')
	if idx == -1 {
		return ec.FullPath()
	}
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()
	// Keep everything after the penultimate separator.
	buf.AppendString(ec.File[idx+1:])
	buf.AppendByte(':')
	buf.AppendInt(int64(ec.Line))
	return buf.String()
}

// A EventPool is a type-safe wrapper around a sync.BytesBufferPool.
type EventPool struct {
	p *sync.Pool
}

// NewEventPool constructs a new BytesBufferPool.
func NewEventPool() EventPool {
	return EventPool{
		p: &sync.Pool{
			New: func() interface{} {
				return &Event{}
			},
		},
	}
}

// Get retrieves a Event from the pool, creating one if necessary.
func (p EventPool) Get() *Event {
	e := p.p.Get().(*Event)

	e.buffer = buffer.GlobalBytesPool().Get()
	e.pool = p

	e.level = MuteLevel
	e.lw = nil

	e.ctx = nil
	e.ctxKeys = nil

	e.isEnabled = false
	e.isCallerEnabled = false

	return e
}

func (p EventPool) put(event *Event) {
	event.buffer.Free()

	event.ctx = nil
	event.ctxKeys = nil

	p.p.Put(event)
}

// Event represents a log event. It is instanced by one of the with method of
// logger and finalized by the log method such as Debug().
type Event struct {
	buffer *buffer.BytesBuffer
	pool   EventPool

	level Level
	lw    LevelWriter

	ctx        context.Context
	ctxKeys    *[]interface{}
	ctxKeysMap *map[interface{}]string

	isEnabled       bool
	isCallerEnabled bool
}

// Free returns the Event to its EventPool.
// Callers must not retain references to the Event after calling Free.
func (e *Event) Free() {
	e.pool.put(e)
}

// Message writes the *Event to level writer.
//
// NOTICE: Once this method is called, the *Event should be disposed.
// Calling twice can have unexpected result.
func (e *Event) Message(message string) {
	if !e.isEnabled {
		return
	}
	e.write(message)
}

// Messagef writes the *Event to level writer.
//
// NOTICE: Once this method is called, the *Event should be disposed.
// Calling twice can have unexpected result.
func (e *Event) Messagef(format string, v ...interface{}) {
	if !e.isEnabled {
		return
	}
	e.write(format, v...)
}

// Byte appends string key and byte value to event.
func (e *Event) Byte(key string, value byte) *Event {
	return e.appendField(key, func() { e.buffer.AppendByte(value) })
}

// Bytes appends string key and bytes value to event.
func (e *Event) Bytes(key string, value []byte) *Event {
	return e.appendField(key, func() {
		if needsQuote(string(value)) {
			e.buffer.AppendString(strconv.Quote(string(value)))
		} else {
			e.buffer.AppendBytes(value)
		}
	})
}

// String appends string key and string value to event.
func (e *Event) String(key string, value string) *Event {
	return e.appendField(key, func() {
		if needsQuote(string(value)) {
			e.buffer.AppendString(strconv.Quote(value))
		} else {
			e.buffer.AppendString(value)
		}
	})
}

// Int appends string key and int value to event.
func (e *Event) Int(key string, value int) *Event {
	return e.appendField(key, func() { e.buffer.AppendInt(int64(value)) })
}

// Int32 appends string key and int32 value to event.
func (e *Event) Int32(key string, value int32) *Event {
	return e.appendField(key, func() { e.buffer.AppendInt(int64(value)) })
}

// Int64 appends string key and int64 value to event.
func (e *Event) Int64(key string, value int64) *Event {
	return e.appendField(key, func() { e.buffer.AppendInt(value) })
}

// Uint appends string key and uint value to event.
func (e *Event) Uint(key string, value uint) *Event {
	return e.appendField(key, func() { e.buffer.AppendUint(uint64(value)) })
}

// Uint32 appends string key and uint32 value to event.
func (e *Event) Uint32(key string, value uint32) *Event {
	return e.appendField(key, func() { e.buffer.AppendUint(uint64(value)) })
}

// Uint64 appends string key and uint64 value to event.
func (e *Event) Uint64(key string, value uint64) *Event {
	return e.appendField(key, func() { e.buffer.AppendUint(value) })
}

// Float32 appends string key and float32 value to event.
func (e *Event) Float32(key string, value float32) *Event {
	return e.appendField(key, func() { e.buffer.AppendFloat(float64(value), 32) })
}

// Float64 appends string key and float value to event.
func (e *Event) Float64(key string, value float64) *Event {
	return e.appendField(key, func() { e.buffer.AppendFloat(value, 64) })
}

// Bool appends string key and bool value to event.
func (e *Event) Bool(key string, value bool) *Event {
	return e.appendField(key, func() { e.buffer.AppendBool(value) })
}

// Time appends string key and time value to event.
func (e *Event) Time(key string, value time.Time, format string) *Event {
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()

	buf.AppendTime(value, format)
	return e.Bytes(key, buf.Bytes())
}

// Error appends string key and error value to event.
func (e *Event) Error(key string, err error) *Event {
	return e.String(key, err.Error())
}

// Interface appends string key and interface value to event.
func (e *Event) Interface(key string, value interface{}) *Event {
	switch v := value.(type) {
	case byte:
		e.Byte(key, v)
	case []byte:
		e.Bytes(key, v)
	case string:
		e.String(key, v)
	case int:
		e.Int(key, v)
	case int32:
		e.Int32(key, v)
	case int64:
		e.Int64(key, v)
	case uint:
		e.Uint(key, v)
	case uint32:
		e.Uint32(key, v)
	case uint64:
		e.Uint64(key, v)
	case float32:
		e.Float32(key, v)
	case float64:
		e.Float64(key, v)
	case bool:
		e.Bool(key, v)
	case time.Time:
		e.Time(key, v, convert.ISO8601Milli)
	case error:
		e.Error(key, v)
	case nil:
		e.String(key, "nil")
	default:
		panic(fmt.Sprintf("unknown field type: %v", value))
	}
	return e
}

func (e *Event) appendField(key string, appendFunc func()) *Event {
	if !e.isEnabled {
		return e
	}

	// Ignore field with empty key.
	if len(key) <= 0 {
		return e
	}

	// Append space if event field not empty.
	if e.buffer.Len() != 0 {
		e.buffer.AppendByte(' ')
	}

	e.buffer.AppendString(key)
	e.buffer.AppendString("=")

	appendFunc()
	return e
}

func (e *Event) write(format string, v ...interface{}) {
	defer e.Free()

	if !e.isEnabled {
		return
	}

	// Append interested contexts.
	if e.ctx != nil && e.ctxKeys != nil && e.ctxKeysMap != nil {
		for _, key := range *e.ctxKeys {
			if value := e.ctx.Value(key); value != nil {
				e.Interface((*e.ctxKeysMap)[key], e.ctx.Value(key))
			}
		}
	}

	// Append caller.
	if e.isCallerEnabled {
		ec := newEventCaller(runtime.Caller(callerSkipOffset))
		e.String("source", ec.TrimmedPath())
	}

	// Compose and store current log.
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()

	// Format print message.
	if len(v) == 0 {
		fmt.Fprint(buf, format)
	} else {
		fmt.Fprintf(buf, format, v...)
	}

	// Append filed.
	buf.AppendByte(' ')
	buf.AppendBytes(e.buffer.Bytes())

	// Finally write.
	if _, err := e.lw.WriteLevel(e.level, buf.Bytes()); err != nil {
		fmt.Fprintf(os.Stderr, "log: could not write event: %v", err)
	}

	switch e.level {
	case PanicLevel:
		panic(buf.String())
	case FatalLevel:
		os.Exit(1)
	}
}

func newEventCaller(pc uintptr, file string, line int, ok bool) (ec *EventCaller) {
	ec = eventCallerPool.Get()

	if ok {
		ec.PC = pc
		ec.File = file
		ec.Line = line
		ec.Defined = true
	}
	return
}

func newEvent(
	ctx context.Context,
	ctxKeys *[]interface{}, ctxKeysMap *map[interface{}]string,
	level Level, lw LevelWriter,
	isEnabled bool, isCallerEnabled bool,
) (e *Event) {
	e = eventPool.Get()

	e.level = level
	e.lw = lw

	e.ctx = ctx
	e.ctxKeys = ctxKeys
	e.ctxKeysMap = ctxKeysMap

	e.isEnabled = isEnabled
	e.isCallerEnabled = isCallerEnabled
	return
}

func needsQuote(s string) bool {
	for i := range s {
		if s[i] < 0x20 || s[i] > 0x7e || s[i] == ' ' || s[i] == '\\' || s[i] == '"' {
			return true
		}
	}
	return false
}

const callerSkipOffset = 2

// eventCallerPool is a pool of newEvent callers.
var eventCallerPool = NewEventCallerPool()

// eventPool is a pool of events.
var eventPool = NewEventPool()
