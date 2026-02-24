package server

import "errors"

var (
	ErrNoSuchUser    = errors.New("no such user")
	ErrNoSuchAddress = errors.New("no such address")
	ErrNoSuchLabel   = errors.New("no such label")
)
