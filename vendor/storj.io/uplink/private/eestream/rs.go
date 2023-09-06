// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"github.com/vivint/infectious"
)

type rsScheme struct {
	fc               *infectious.FEC
	erasureShareSize int
}

// NewRSScheme returns a Reed-Solomon-based ErasureScheme.
func NewRSScheme(fc *infectious.FEC, erasureShareSize int) ErasureScheme {
	return &rsScheme{fc: fc, erasureShareSize: erasureShareSize}
}

func (s *rsScheme) EncodeSingle(input, output []byte, num int) (err error) {
	return s.fc.EncodeSingle(input, output, num)
}

func (s *rsScheme) Encode(input []byte, output func(num int, data []byte)) (
	err error) {
	return s.fc.Encode(input, func(s infectious.Share) {
		output(s.Number, s.Data)
	})
}

func (s *rsScheme) Decode(out []byte, in map[int][]byte) ([]byte, error) {
	shares := make([]infectious.Share, 0, len(in))
	for num, data := range in {
		shares = append(shares, infectious.Share{Number: num, Data: data})
	}
	return s.fc.Decode(out, shares)
}

func (s *rsScheme) ErasureShareSize() int {
	return s.erasureShareSize
}

func (s *rsScheme) StripeSize() int {
	return s.erasureShareSize * s.fc.Required()
}

func (s *rsScheme) TotalCount() int {
	return s.fc.Total()
}

func (s *rsScheme) RequiredCount() int {
	return s.fc.Required()
}
