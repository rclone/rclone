// (C) 1996-1998 Luigi Rizzo (luigi@iet.unipi.it)
//     2009-2010 Jack Lloyd (lloyd@randombit.net)
//     2011 Billy Brumley (billy.brumley@aalto.fi)
//     2016-2017 Vivint, Inc. (jeff.wendling@vivint.com)
//
// Portions derived from code by Phil Karn (karn@ka9q.ampr.org),
// Robert Morelos-Zaragoza (robert@spectra.eng.hawaii.edu) and Hari
// Thirumoorthy (harit@spectra.eng.hawaii.edu), Aug 1995
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
// 1. Redistributions of source code must retain the above copyright
//    notice, this list of conditions and the following disclaimer.
//
// 2. Redistributions in binary form must reproduce the above copyright
//    notice, this list of conditions and the following disclaimer in the
//    documentation and/or other materials provided with the
//    distribution.
//
// THIS SOFTWARE IS PROVIDED BY THE AUTHORS ``AS IS'' AND ANY EXPRESS OR
// IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
// WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
// DISCLAIMED. IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY DIRECT,
// INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
// (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR
// SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION)
// HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING
// IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE
// POSSIBILITY OF SUCH DAMAGE.

package infectious

import (
	"bytes"
	"errors"
)

type pivotSearcher struct {
	k    int
	ipiv []bool
}

func newPivotSearcher(k int) *pivotSearcher {
	return &pivotSearcher{
		k:    k,
		ipiv: make([]bool, k),
	}
}

func (p *pivotSearcher) search(col int, matrix []byte) (int, int, error) {
	if p.ipiv[col] == false && matrix[col*p.k+col] != 0 {
		p.ipiv[col] = true
		return col, col, nil
	}

	for row := 0; row < p.k; row++ {
		if p.ipiv[row] {
			continue
		}

		for i := 0; i < p.k; i++ {
			if p.ipiv[i] == false && matrix[row*p.k+i] != 0 {
				p.ipiv[i] = true
				return row, i, nil
			}
		}
	}

	return 0, 0, errors.New("pivot not found")
}

func swap(a, b *byte) {
	tmp := *a
	*a = *b
	*b = tmp
}

// TODO(jeff): matrix is a K*K array, row major.
func invertMatrix(matrix []byte, k int) error {
	pivot_searcher := newPivotSearcher(k)
	indxc := make([]int, k)
	indxr := make([]int, k)
	id_row := make([]byte, k)

	for col := 0; col < k; col++ {
		icol, irow, err := pivot_searcher.search(col, matrix)
		if err != nil {
			return err
		}

		if irow != icol {
			for i := 0; i < k; i++ {
				swap(&matrix[irow*k+i], &matrix[icol*k+i])
			}
		}

		indxr[col] = irow
		indxc[col] = icol
		pivot_row := matrix[icol*k:][:k]
		c := pivot_row[icol]

		if c == 0 {
			return errors.New("singular matrix")
		}

		if c != 1 {
			c = gf_inverse[c]
			pivot_row[icol] = 1
			mul_c := gf_mul_table[c][:]

			for i := 0; i < k; i++ {
				pivot_row[i] = mul_c[pivot_row[i]]
			}
		}

		id_row[icol] = 1
		if !bytes.Equal(pivot_row, id_row) {
			p := matrix
			for i := 0; i < k; i++ {
				if i != icol {
					c = p[icol]
					p[icol] = 0
					addmul(p[:k], pivot_row, c)
				}
				p = p[k:]
			}
		}

		id_row[icol] = 0
	}

	for i := 0; i < k; i++ {
		if indxr[i] != indxc[i] {
			for row := 0; row < k; row++ {
				swap(&matrix[row*k+indxr[i]], &matrix[row*k+indxc[i]])
			}
		}
	}
	return nil
}

func createInvertedVdm(vdm []byte, k int) {
	if k == 1 {
		vdm[0] = 1
		return
	}

	b := make([]byte, k)
	c := make([]byte, k)

	c[k-1] = 0
	for i := 1; i < k; i++ {
		mul_p_i := gf_mul_table[gf_exp[i]][:]
		for j := k - 1 - (i - 1); j < k-1; j++ {
			c[j] ^= mul_p_i[c[j+1]]
		}
		c[k-1] ^= gf_exp[i]
	}

	for row := 0; row < k; row++ {
		index := 0
		if row != 0 {
			index = int(gf_exp[row])
		}
		mul_p_row := gf_mul_table[index][:]

		t := byte(1)
		b[k-1] = 1
		for i := k - 2; i >= 0; i-- {
			b[i] = c[i+1] ^ mul_p_row[b[i+1]]
			t = b[i] ^ mul_p_row[t]
		}

		mul_t_inv := gf_mul_table[gf_inverse[t]][:]
		for col := 0; col < k; col++ {
			vdm[col*k+row] = mul_t_inv[b[col]]
		}
	}
}
