package api

import "errors"

var ErrBadRequest = errors.New("Bad Request")
var ErrForbidden = errors.New("Forbidden")
var ErrUnauthorized = errors.New("Unauthorized")
var ErrTooManyRequests = errors.New("Too Many Requests")
var ErrServer = errors.New("Server Error")
var ErrNotFound = errors.New("Not Found")
var ErrUndefined = errors.New("Undefined Error")