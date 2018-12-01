package logger

import "io"

// Outable interface for loggers that allow setting the output writer
type Outable interface {
	SetOutput(out io.Writer)
}
