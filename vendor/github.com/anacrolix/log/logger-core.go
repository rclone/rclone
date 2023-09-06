package log

import (
	"fmt"
	"path/filepath"
)

// loggerCore is the essential part of Logger.
type loggerCore struct {
	nonZero bool
	names   []string
	values  []interface{}
	// Propagate on NOTSET?
	defaultLevel Level
	// Use propagation on NOTSET.
	filterLevel Level
	msgMaps     []func(Msg) Msg
	Handlers    []Handler
}

func (l loggerCore) asLogger() Logger {
	return Logger{l}
}

// Returns a logger that adds the given values to logged messages.
func (l loggerCore) WithValues(v ...interface{}) Logger {
	l.values = append(l.values, v...)
	return l.asLogger()
}

// Returns a logger that for a given message propagates the result of `f` instead.
func (l loggerCore) WithMap(f func(m Msg) Msg) Logger {
	l.msgMaps = append(l.msgMaps, f)
	return l.asLogger()
}

func (l loggerCore) WithDefaultLevel(level Level) Logger {
	l.defaultLevel = level
	return l.asLogger()
}

func (l loggerCore) WithFilterLevel(minLevel Level) Logger {
	l.filterLevel = minLevel
	return l.asLogger()
}

// Deprecated. Use WithFilterLevel. This method name is misleading and doesn't follow the convention
// elsewhere.
func (l loggerCore) FilterLevel(minLevel Level) Logger {
	return l.WithFilterLevel(minLevel)
}

func (l loggerCore) IsZero() bool {
	return !l.nonZero
}

// Deprecated. This should require a msg, since filtering includes the location a msg is created at.
// That would require building a message before we can do checks, which means lazily constructing
// the message but not the caller location.
func (l loggerCore) IsEnabledFor(level Level) bool {
	// TODO: Take a message?
	return true
}

func (l loggerCore) LazyLog(level Level, f func() Msg) {
	l.lazyLog(level, 1, f)
}

func (l loggerCore) LazyLogDefaultLevel(f func() Msg) {
	l.lazyLog(NotSet, 1, f)
}

func (l loggerCore) lazyLog(level Level, skip int, f func() Msg) {
	if level.isNotSet() {
		level = l.defaultLevel
	}
	r := f().Skip(skip + 1)
	msgLoc := getMsgLogLoc(r)
	names := append(
		l.names[:len(l.names):len(l.names)],
		msgLoc.Package,
		fmt.Sprintf("%v:%v", filepath.Base(msgLoc.File), msgLoc.Line),
	)
	if rulesLevel, ok := levelFromRules(names); ok {
		if level.LessThan(rulesLevel) {
			return
		}
	} else if level.LessThan(l.filterLevel) {
		return
	}
	for i := len(l.msgMaps) - 1; i >= 0; i-- {
		r = l.msgMaps[i](r)
	}
	l.handle(level, r, names)
}

// Goes from an affirmative decision to log, to sending it to the handlers in the right form.
func (l loggerCore) handle(level Level, m Msg, names []string) {
	r := Record{
		// Do we really need to be passing the full Msg caller context at this point?
		Msg:   m.Skip(1),
		Level: level,
		Names: names,
	}
	if !l.nonZero {
		panic(fmt.Sprintf("Logger uninitialized. names=%q", l.names))
	}
	for _, h := range l.Handlers {
		h.Handle(r)
	}
}

func (l loggerCore) WithNames(names ...string) Logger {
	// Avoid sharing after appending. This might not be enough because some formatters might add
	// more elements concurrently, or names could be empty.
	l.names = append(l.names[:len(l.names):len(l.names)], names...)
	return l.asLogger()
}

// Clobber the Loggers Handlers. Note this breaks convention by not returning a new Logger, but
// seems to fit here.
func (l *loggerCore) SetHandlers(h ...Handler) {
	l.Handlers = h
	l.nonZero = true
}
