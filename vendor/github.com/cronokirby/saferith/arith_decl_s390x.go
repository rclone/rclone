// Copyright 2016 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE_go file.

//go:build !math_big_pure_go
// +build !math_big_pure_go

package saferith

func addVV_check(z, x, y []Word) (c Word)
func addVV_vec(z, x, y []Word) (c Word)
func addVV_novec(z, x, y []Word) (c Word)
func subVV_check(z, x, y []Word) (c Word)
func subVV_vec(z, x, y []Word) (c Word)
func subVV_novec(z, x, y []Word) (c Word)

// This should be feature detected, but we can't use the internal/cpu package
var hasVX = false
