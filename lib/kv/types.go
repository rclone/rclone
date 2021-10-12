package kv

import (
	"context"

	"github.com/pkg/errors"
)

// package errors
var (
	ErrEmpty       = errors.New("database empty")
	ErrInactive    = errors.New("database stopped")
	ErrUnsupported = errors.New("unsupported on this OS")
)

// Op represents a database operation
type Op interface {
	Do(context.Context, Bucket) error
}

// Bucket decouples bbolt.Bucket from key-val operations
type Bucket interface {
	Get([]byte) []byte
	Put([]byte, []byte) error
	Delete([]byte) error
	ForEach(func(bkey, data []byte) error) error
	Cursor() Cursor
}

// Cursor decouples bbolt.Cursor from key-val operations
type Cursor interface {
	First() ([]byte, []byte)
	Next() ([]byte, []byte)
	Seek([]byte) ([]byte, []byte)
}
