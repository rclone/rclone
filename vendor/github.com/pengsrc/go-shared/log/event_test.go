package log

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/pengsrc/go-shared/buffer"
	"github.com/pengsrc/go-shared/convert"
)

func TestEventCallerPool(t *testing.T) {
	p := NewEventCallerPool()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				eventCaller := p.Get()
				assert.NotNil(t, eventCaller)
				eventCaller.Free()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestEntryCaller(t *testing.T) {
	tests := []struct {
		caller *EventCaller
		full   string
		short  string
	}{
		{
			caller: newEventCaller(100, "/path/to/foo.go", 42, false),
			full:   "undefined",
			short:  "undefined",
		},
		{
			caller: newEventCaller(100, "/path/to/foo.go", 42, true),
			full:   "/path/to/foo.go:42",
			short:  "to/foo.go:42",
		},
		{
			caller: newEventCaller(100, "to/foo.go", 42, true),
			full:   "to/foo.go:42",
			short:  "to/foo.go:42",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.full, tt.caller.String(), "Unexpected string from EntryCaller.")
		assert.Equal(t, tt.full, tt.caller.FullPath(), "Unexpected FullPath from EntryCaller.")
		assert.Equal(t, tt.short, tt.caller.TrimmedPath(), "Unexpected TrimmedPath from EntryCaller.")
	}
}

func TestEventPool(t *testing.T) {
	p := NewEventPool()

	var wg sync.WaitGroup
	for g := 0; g < 10; g++ {
		wg.Add(1)
		go func() {
			for i := 0; i < 100; i++ {
				event := p.Get()
				assert.NotNil(t, event)
				event.Free()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestEvent(t *testing.T) {
	buf := buffer.GlobalBytesPool().Get()
	defer buf.Free()

	l, err := NewLogger(buf, "DEBUG")
	assert.NoError(t, err)

	l.DebugEvent(context.TODO()).Byte("b", 'b').Message("DEBUG b")
	assert.Contains(t, buf.String(), "DEBUG b b=b")
	t.Log(buf.String())
	buf.Reset()

	l.DebugEvent(context.TODO()).Bytes("bs", []byte("bs")).Message("DEBUG bs")
	l.DebugEvent(context.TODO()).Bytes("bs", []byte("bs bs")).Messagef("DEBUG %s", "bs")
	assert.Contains(t, buf.String(), "DEBUG bs bs=bs")
	assert.Contains(t, buf.String(), `DEBUG bs bs="bs bs"`)
	buf.Reset()

	l.DebugEvent(context.TODO()).String("s", "s").Message("DEBUG s")
	l.DebugEvent(context.TODO()).String("s", "s s").Messagef("DEBUG %d", 1024)
	assert.Contains(t, buf.String(), "DEBUG s s=s")
	assert.Contains(t, buf.String(), `DEBUG 1024 s="s s"`)
	buf.Reset()

	l.InfoEvent(context.TODO()).
		Int("i", 1).Int32("i32", int32(2)).Int64("i64", int64(3)).
		Messagef("INFO %d", 123)
	assert.Contains(t, buf.String(), "INFO 123 i=1 i32=2 i64=3")
	buf.Reset()

	l.InfoEvent(context.TODO()).
		Uint("i", 1).Uint32("i32", uint32(2)).Uint64("i64", uint64(3)).
		Messagef("INFO %d", 123)
	assert.Contains(t, buf.String(), "INFO 123 i=1 i32=2 i64=3")
	buf.Reset()

	l.WarnEvent(context.TODO()).
		Float32("f32", float32(32.2333)).Float64("f64", float64(64.6444)).
		Messagef("WARN %s %d.", "hello", 1024)
	assert.Contains(t, buf.String(), "WARN hello 1024. f32=32.2333 f64=64.6444")
	buf.Reset()

	l.WarnEvent(context.TODO()).
		Bool("true", true).Bool("false", false).
		Message("WARN bool.")
	assert.Contains(t, buf.String(), "WARN bool. true=true false=false")
	buf.Reset()

	l.ErrorEvent(context.TODO()).
		Time("time", time.Time{}, convert.RFC822).Error("error", errors.New("error message")).
		Message("Error.")
	assert.Contains(t, buf.String(), `Error. time="Mon, 01 Jan 0001 00:00:00 GMT" error="error message"`)
	buf.Reset()

	l.DebugEvent(context.TODO()).
		String("a", "a a").
		String("b", "b'b").
		String("c", `c"c`).
		Message("yes")
	assert.Contains(t, buf.String(), `a="a a"`)
	assert.Contains(t, buf.String(), `b=b'b`)
	assert.Contains(t, buf.String(), `c="c\"c"`)
	buf.Reset()
}
