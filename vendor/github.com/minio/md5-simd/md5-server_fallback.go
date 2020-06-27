//+build !amd64 appengine !gc noasm

// Copyright (c) 2020 MinIO Inc. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package md5simd

// NewServer - Create new object for parallel processing handling
func NewServer() *fallbackServer {
	return &fallbackServer{}
}
