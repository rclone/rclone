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
	"sort"
)

// Decode will take a destination buffer (can be nil) and a list of shares
// (pieces). It will return the data passed in to the corresponding Encode
// call or return an error.
//
// It will first correct the shares using Correct, mutating and reordering the
// passed-in shares arguments. Then it will rebuild the data using Rebuild.
// Finally it will concatenate the data into the given output buffer dst if it
// has capacity, growing it otherwise.
//
// If you already know your data does not contain errors, Rebuild will be
// faster.
//
// If you only want to identify which pieces are bad, you may be interested in
// Correct.
//
// If you don't want the data concatenated for you, you can use Correct and
// then Rebuild individually.
func (f *FEC) Decode(dst []byte, shares []Share) ([]byte, error) {
	err := f.Correct(shares)
	if err != nil {
		return nil, err
	}

	if len(shares) == 0 {
		return nil, errors.New("must specify at least one share")
	}
	piece_len := len(shares[0].Data)
	result_len := piece_len * f.k
	if cap(dst) < result_len {
		dst = make([]byte, result_len)
	} else {
		dst = dst[:result_len]
	}

	return dst, f.Rebuild(shares, func(s Share) {
		copy(dst[s.Number*piece_len:], s.Data)
	})
}

func (f *FEC) decode(shares []Share, output func(Share)) error {
	err := f.Correct(shares)
	if err != nil {
		return err
	}
	return f.Rebuild(shares, output)
}

// Correct implements the Berlekamp-Welch algorithm for correcting
// errors in given FEC encoded data. It will correct the supplied shares,
// mutating the underlying byte slices and reordering the shares
func (fc *FEC) Correct(shares []Share) error {
	if len(shares) < fc.k {
		return errors.New("must specify at least the number of required shares")
	}

	sort.Sort(byNumber(shares))

	// fast path: check to see if there are no errors by evaluating it with
	// the syndrome matrix.
	synd, err := fc.syndromeMatrix(shares)
	if err != nil {
		return err
	}
	buf := make([]byte, len(shares[0].Data))

	for i := 0; i < synd.r; i++ {
		for j := range buf {
			buf[j] = 0
		}

		for j := 0; j < synd.c; j++ {
			addmul(buf, shares[j].Data, byte(synd.get(i, j)))
		}

		for j := range buf {
			if buf[j] == 0 {
				continue
			}
			data, err := fc.berlekampWelch(shares, j)
			if err != nil {
				return err
			}
			for _, share := range shares {
				share.Data[j] = data[share.Number]
			}
		}
	}

	return nil
}

func (fc *FEC) berlekampWelch(shares []Share, index int) ([]byte, error) {
	k := fc.k        // required size
	r := len(shares) // required + redundancy size
	e := (r - k) / 2 // deg of E polynomial
	q := e + k       // def of Q polynomial

	if e <= 0 {
		return nil, NotEnoughShares
	}

	const interp_base = gfVal(2)

	eval_point := func(num int) gfVal {
		if num == 0 {
			return 0
		}
		return interp_base.pow(num - 1)
	}

	dim := q + e

	// build the system of equations s * u = f
	s := matrixNew(dim, dim) // constraint matrix
	a := matrixNew(dim, dim) // augmented matrix
	f := make(gfVals, dim)   // constant column vector
	u := make(gfVals, dim)   // solution vector

	for i := 0; i < dim; i++ {
		x_i := eval_point(shares[i].Number)
		r_i := gfConst(shares[i].Data[index])

		f[i] = x_i.pow(e).mul(r_i)

		for j := 0; j < q; j++ {
			s.set(i, j, x_i.pow(j))
			if i == j {
				a.set(i, j, gfConst(1))
			}
		}

		for k := 0; k < e; k++ {
			j := k + q

			s.set(i, j, x_i.pow(k).mul(r_i))
			if i == j {
				a.set(i, j, gfConst(1))
			}
		}
	}

	// invert and put the result in a
	err := s.invertWith(a)
	if err != nil {
		return nil, err
	}

	// multiply the inverted matrix by the column vector
	for i := 0; i < dim; i++ {
		ri := a.indexRow(i)
		u[i] = ri.dot(f)
	}

	// reverse u for easier construction of the polynomials
	for i := 0; i < len(u)/2; i++ {
		o := len(u) - i - 1
		u[i], u[o] = u[o], u[i]
	}

	q_poly := gfPoly(u[e:])
	e_poly := append(gfPoly{gfConst(1)}, u[:e]...)

	p_poly, rem, err := q_poly.div(e_poly)
	if err != nil {
		return nil, err
	}

	if !rem.isZero() {
		return nil, TooManyErrors
	}

	out := make([]byte, fc.n)
	for i := range out {
		pt := gfConst(0)
		if i != 0 {
			pt = interp_base.pow(i - 1)
		}
		out[i] = byte(p_poly.eval(pt))
	}

	return out, nil
}

func (fc *FEC) syndromeMatrix(shares []Share) (gfMat, error) {
	// get a list of keepers
	keepers := make([]bool, fc.n)
	shareCount := 0
	for _, share := range shares {
		if !keepers[share.Number] {
			keepers[share.Number] = true
			shareCount++
		}
	}

	// create a vandermonde matrix but skip columns where we're missing the
	// share.
	out := matrixNew(fc.k, shareCount)
	for i := 0; i < fc.k; i++ {
		skipped := 0
		for j := 0; j < fc.n; j++ {
			if !keepers[j] {
				skipped++
				continue
			}

			out.set(i, j-skipped, gfConst(fc.vand_matrix[i*fc.n+j]))
		}
	}

	// standardize the output and convert into parity form
	err := out.standardize()
	if err != nil {
		return gfMat{}, err
	}

	return out.parity(), nil
}
