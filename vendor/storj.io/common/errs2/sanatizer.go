// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package errs2

import (
	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/rpc/rpcstatus"
)

// CodeMap is used to apply the correct rpc status code to error classes.
type CodeMap map[*errs.Class]rpcstatus.StatusCode

// LoggingSanitizer consolidates logging of original errors with sanitization of internal errors.
type LoggingSanitizer struct {
	wrapper *errs.Class
	log     *zap.Logger
	codeMap CodeMap
}

// NewLoggingSanitizer creates a new LoggingSanitizer.
func NewLoggingSanitizer(wrapper *errs.Class, log *zap.Logger, codeMap CodeMap) *LoggingSanitizer {
	return &LoggingSanitizer{
		wrapper: wrapper,
		log:     log,
		codeMap: codeMap,
	}
}

// Error logs the message and error to the logger and returns the sanitized error.
func (sanitizer *LoggingSanitizer) Error(msg string, err error) error {
	if sanitizer.wrapper != nil {
		err = sanitizer.wrapper.Wrap(err)
	}

	if sanitizer.log != nil {
		sanitizer.log.Error(msg, zap.Error(err))
	}

	for errClass, code := range sanitizer.codeMap {
		if errClass.Has(err) {
			return rpcstatus.Error(code, err.Error())
		}
	}

	if sanitizer.wrapper == nil {
		return rpcstatus.Error(rpcstatus.Internal, msg)
	}
	return rpcstatus.Error(rpcstatus.Internal, sanitizer.wrapper.New(msg).Error())
}
