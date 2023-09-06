package log

import "errors"

type errorWithLevel struct {
	Level Level
	error
}

func ErrorLevel(err error) Level {
	var withLevel errorWithLevel
	if !errors.As(err, &withLevel) {
		return NotSet
	}
	return withLevel.Level
}

func WithLevel(level Level, err error) error {
	return errorWithLevel{level, err}
}
