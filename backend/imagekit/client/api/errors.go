package api

import "errors"

var ErrBadRequest = errors.New("bad request")
var ErrForbidden = errors.New("forbidden")
var ErrUnauthorized = errors.New("unauthorized")
var ErrTooManyRequests = errors.New("too many requests")
var ErrServer = errors.New("server error")
var ErrNotFound = errors.New("not found")
var ErrUndefined = errors.New("undefined error")
