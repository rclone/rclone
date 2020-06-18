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

package infectious

import (
	"errors"
	"fmt"
	"strings"
	"unsafe"
)

//
// basic helpers around gf(2^8) values
//

type gfVal byte

func gfConst(val byte) gfVal {
	return gfVal(val)
}

func (b gfVal) pow(val int) gfVal {
	out := gfVal(1)
	mul_base := gf_mul_table[b][:]
	for i := 0; i < val; i++ {
		out = gfVal(mul_base[out])
	}
	return out
}

func (a gfVal) mul(b gfVal) gfVal {
	return gfVal(gf_mul_table[a][b])
}

func (a gfVal) div(b gfVal) (gfVal, error) {
	if b == 0 {
		return 0, errors.New("divide by zero")
	}
	if a == 0 {
		return 0, nil
	}
	return gfVal(gf_exp[gf_log[a]-gf_log[b]]), nil
}

func (a gfVal) add(b gfVal) gfVal {
	return gfVal(a ^ b)
}

func (a gfVal) isZero() bool {
	return a == 0
}

func (a gfVal) inv() (gfVal, error) {
	if a == 0 {
		return 0, errors.New("invert zero")
	}
	return gfVal(gf_exp[255-gf_log[a]]), nil
}

//
// basic helpers about a slice of gf(2^8) values
//

type gfVals []gfVal

func (a gfVals) unsafeBytes() []byte {
	return *(*[]byte)(unsafe.Pointer(&a))
}

func (a gfVals) dot(b gfVals) gfVal {
	out := gfConst(0)
	for i := range a {
		out = out.add(a[i].mul(b[i]))
	}
	return out
}

func (a gfVals) String() string {
	return fmt.Sprintf("%02x", a.unsafeBytes())
}

//
// basic helpers for dealing with polynomials with coefficients in gf(2^8)
//

type gfPoly []gfVal

func polyZero(size int) gfPoly {
	out := make(gfPoly, size)
	for i := range out {
		out[i] = gfConst(0)
	}
	return out
}

func (p gfPoly) isZero() bool {
	for _, coef := range p {
		if !coef.isZero() {
			return false
		}
	}
	return true
}

func (p gfPoly) deg() int {
	return len(p) - 1
}

func (p gfPoly) index(power int) gfVal {
	if power < 0 {
		return gfConst(0)
	}
	which := p.deg() - power
	if which < 0 {
		return gfConst(0)
	}
	return p[which]
}

func (p gfPoly) scale(factor gfVal) gfPoly {
	out := make(gfPoly, len(p))
	for i, coef := range p {
		out[i] = coef.mul(factor)
	}
	return out
}

func (p *gfPoly) set(pow int, coef gfVal) {
	which := p.deg() - pow
	if which < 0 {
		*p = append(polyZero(-which), *p...)
		which = p.deg() - pow
	}
	(*p)[which] = coef
}

func (p gfPoly) add(b gfPoly) gfPoly {
	size := len(p)
	if lb := len(b); lb > size {
		size = lb
	}
	out := make(gfPoly, size)
	for i := range out {
		pi := p.index(i)
		bi := b.index(i)
		out.set(i, pi.add(bi))
	}
	return out
}

func (p gfPoly) div(b gfPoly) (q, r gfPoly, err error) {
	// sanitize the divisor by removing leading zeros.
	for len(b) > 0 && b[0].isZero() {
		b = b[1:]
	}
	if len(b) == 0 {
		return nil, nil, errors.New("divide by zero")
	}

	// sanitize the base poly as well
	for len(p) > 0 && p[0].isZero() {
		p = p[1:]
	}
	if len(p) == 0 {
		return polyZero(1), polyZero(1), nil
	}

	const debug = false
	indent := 2*len(b) + 1

	if debug {
		fmt.Printf("%02x %02x\n", b, p)
	}

	for b.deg() <= p.deg() {
		leading_p := p.index(p.deg())
		leading_b := b.index(b.deg())

		if debug {
			fmt.Printf("leading_p: %02x leading_b: %02x\n",
				leading_p, leading_b)
		}

		coef, err := leading_p.div(leading_b)
		if err != nil {
			return nil, nil, err
		}

		if debug {
			fmt.Printf("coef: %02x\n", coef)
		}

		q = append(q, coef)

		scaled := b.scale(coef)
		padded := append(scaled, polyZero(p.deg()-scaled.deg())...)

		if debug {
			fmt.Printf("%s%02x\n", strings.Repeat(" ", indent), padded)
			indent += 2
		}

		p = p.add(padded)
		if !p[0].isZero() {
			return nil, nil, fmt.Errorf("alg error: %x", p)
		}
		p = p[1:]
	}

	for len(p) > 1 && p[0].isZero() {
		p = p[1:]
	}

	return q, p, nil
}

func (p gfPoly) eval(x gfVal) gfVal {
	out := gfConst(0)
	for i := 0; i <= p.deg(); i++ {
		x_i := x.pow(i)
		p_i := p.index(i)
		out = out.add(p_i.mul(x_i))
	}
	return out
}

//
// basic helpers for matrices in gf(2^8)
//

type gfMat struct {
	d    gfVals
	r, c int
}

func matrixNew(i, j int) gfMat {
	return gfMat{
		d: make(gfVals, i*j),
		r: i, c: j,
	}
}

func (m gfMat) String() (out string) {
	if m.r == 0 {
		return ""
	}

	for i := 0; i < m.r-1; i++ {
		out += fmt.Sprintln(m.indexRow(i))
	}
	out += fmt.Sprint(m.indexRow(m.r - 1))

	return out
}

func (m gfMat) index(i, j int) int {
	return m.c*i + j
}

func (m gfMat) get(i, j int) gfVal {
	return m.d[m.index(i, j)]
}

func (m gfMat) set(i, j int, val gfVal) {
	m.d[m.index(i, j)] = val
}

func (m gfMat) indexRow(i int) gfVals {
	return m.d[m.index(i, 0):m.index(i+1, 0)]
}

func (m gfMat) swapRow(i, j int) {
	tmp := make(gfVals, m.r)
	ri := m.indexRow(i)
	rj := m.indexRow(j)
	copy(tmp, ri)
	copy(ri, rj)
	copy(rj, tmp)
}

func (m gfMat) scaleRow(i int, val gfVal) {
	ri := m.indexRow(i)
	for i := range ri {
		ri[i] = ri[i].mul(val)
	}
}

func (m gfMat) addmulRow(i, j int, val gfVal) {
	ri := m.indexRow(i)
	rj := m.indexRow(j)
	addmul(rj.unsafeBytes(), ri.unsafeBytes(), byte(val))
}

// in place invert. the output is put into a and m is turned into the identity
// matrix. a is expected to be the identity matrix.
func (m gfMat) invertWith(a gfMat) error {
	for i := 0; i < m.r; i++ {
		p_row, p_val := i, m.get(i, i)
		for j := i + 1; j < m.r && p_val.isZero(); j++ {
			p_row, p_val = j, m.get(j, i)
		}
		if p_val.isZero() {
			continue
		}

		if p_row != i {
			m.swapRow(i, p_row)
			a.swapRow(i, p_row)
		}

		inv, err := p_val.inv()
		if err != nil {
			return err
		}
		m.scaleRow(i, inv)
		a.scaleRow(i, inv)

		for j := i + 1; j < m.r; j++ {
			leading := m.get(j, i)
			m.addmulRow(i, j, leading)
			a.addmulRow(i, j, leading)
		}
	}

	for i := m.r - 1; i > 0; i-- {
		for j := i - 1; j >= 0; j-- {
			trailing := m.get(j, i)
			m.addmulRow(i, j, trailing)
			a.addmulRow(i, j, trailing)
		}
	}

	return nil
}

// in place standardize.
func (m gfMat) standardize() error {
	for i := 0; i < m.r; i++ {
		p_row, p_val := i, m.get(i, i)
		for j := i + 1; j < m.r && p_val.isZero(); j++ {
			p_row, p_val = j, m.get(j, i)
		}
		if p_val.isZero() {
			continue
		}

		if p_row != i {
			m.swapRow(i, p_row)
		}

		inv, err := p_val.inv()
		if err != nil {
			return err
		}
		m.scaleRow(i, inv)

		for j := i + 1; j < m.r; j++ {
			leading := m.get(j, i)
			m.addmulRow(i, j, leading)
		}
	}

	for i := m.r - 1; i > 0; i-- {
		for j := i - 1; j >= 0; j-- {
			trailing := m.get(j, i)
			m.addmulRow(i, j, trailing)
		}
	}

	return nil
}

// parity returns the new matrix because it changes dimensions and stuff. it
// can be done in place, but is easier to implement with a copy.
func (m gfMat) parity() gfMat {
	// we assume m is in standard form already
	// it is of form [I_r | P]
	// our output will be [-P_transpose | I_(c - r)]
	// but our field is of characteristic 2 so we do not need the negative.

	// In terms of m:
	// I_r has r rows and r columns.
	// P has r rows and c-r columns.
	// P_transpose has c-r rows, and r columns.
	// I_(c-r) has c-r rows and c-r columns.
	// so: out.r == c-r, out.c == r + c - r == c

	out := matrixNew(m.c-m.r, m.c)

	// step 1. fill in the identity. it starts at column offset r.
	for i := 0; i < m.c-m.r; i++ {
		out.set(i, i+m.r, gfConst(1))
	}

	// step 2: fill in the transposed P matrix. i and j are in terms of out.
	for i := 0; i < m.c-m.r; i++ {
		for j := 0; j < m.r; j++ {
			out.set(i, j, m.get(j, i+m.r))
		}
	}

	return out
}
