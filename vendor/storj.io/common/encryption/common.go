// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

// Package encryption collects common cryptographic primitives needed for path and data encryption.
package encryption

import (
	"github.com/zeebo/errs"
)

// Error is the default encryption errs class
var Error = errs.Class("encryption error")

// ErrDecryptFailed is the errs class when the decryption fails
var ErrDecryptFailed = errs.Class("decryption failed, check encryption key")

// ErrInvalidConfig is the errs class for invalid configuration
var ErrInvalidConfig = errs.Class("invalid encryption configuration")
