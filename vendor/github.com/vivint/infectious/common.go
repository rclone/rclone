// The MIT License (MIT)
//
// Copyright (C) 2016-2017 Vivint, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

// Package infectious implements Reed-Solomon forward error correction [1]. It
// uses the Berlekamp-Welch [2] error correction algorithm to achieve the
// ability to actually correct errors.
//
// Caution: this package API leans toward providing the user more power and
// performance at the expense of having some really sharp edges! Read the
// documentation about memory lifecycles carefully!
//
// We wrote a blog post about how this library works!
// https://innovation.vivint.com/introduction-to-reed-solomon-bc264d0794f8
//
//   [1] https://en.wikipedia.org/wiki/Reed%E2%80%93Solomon_error_correction
//   [2] https://en.wikipedia.org/wiki/Berlekamp%E2%80%93Welch_algorithm
package infectious

import (
	"errors"

	"golang.org/x/sys/cpu"
)

var hasAVX2 = cpu.X86.HasAVX2
var hasSSSE3 = cpu.X86.HasSSSE3

var (
	NotEnoughShares = errors.New("not enough shares")
	TooManyErrors   = errors.New("too many errors to reconstruct")
)
