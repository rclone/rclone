// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

// Package hmacsha512 contains an inlined an optimized version of hmac+sha512.
// Unfortunately, this requires exposing some of the details from crypto/sha512.
package hmacsha512

// Currently vendored crypto/sha512 version is go1.19.1
