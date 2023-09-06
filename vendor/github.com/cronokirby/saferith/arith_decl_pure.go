// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE_go file.

//go:build math_big_pure_go
// +build math_big_pure_go

package saferith

func mulWW(x, y Word) (z1, z0 Word) {
	return mulWW_g(x, y)
}

func addVV(z, x, y []Word) (c Word) {
	return addVV_g(z, x, y)
}

func subVV(z, x, y []Word) (c Word) {
	return subVV_g(z, x, y)
}

func addVW(z, x []Word, y Word) (c Word) {
	return addVW_g(z, x, y)
}

func subVW(z, x []Word, y Word) (c Word) {
	return subVW_g(z, x, y)
}

func shlVU(z, x []Word, s uint) (c Word) {
	return shlVU_g(z, x, s)
}

func shrVU(z, x []Word, s uint) (c Word) {
	return shrVU_g(z, x, s)
}

func mulAddVWW(z, x []Word, y, r Word) (c Word) {
	return mulAddVWW_g(z, x, y, r)
}

func addMulVVW(z, x []Word, y Word) (c Word) {
	return addMulVVW_g(z, x, y)
}
