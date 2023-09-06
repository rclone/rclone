// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package eestream

import (
	"github.com/vivint/infectious"
)

type unsafeRSScheme struct {
	fc               *infectious.FEC
	erasureShareSize int
}

// NewUnsafeRSScheme returns a Reed-Solomon-based ErasureScheme without error correction.
func NewUnsafeRSScheme(fc *infectious.FEC, erasureShareSize int) ErasureScheme {
	return &unsafeRSScheme{fc: fc, erasureShareSize: erasureShareSize}
}

func (s *unsafeRSScheme) EncodeSingle(input, output []byte, num int) (err error) {
	return s.fc.EncodeSingle(input, output, num)
}

func (s *unsafeRSScheme) Encode(input []byte, output func(num int, data []byte)) (
	err error) {
	return s.fc.Encode(input, func(s infectious.Share) {
		output(s.Number, s.Data)
	})
}

func (s *unsafeRSScheme) Decode(out []byte, in map[int][]byte) ([]byte, error) {
	shares := make([]infectious.Share, 0, len(in))
	for num, data := range in {
		shares = append(shares, infectious.Share{Number: num, Data: data})
	}

	stripe := make([]byte, s.RequiredCount()*s.ErasureShareSize())
	err := s.fc.Rebuild(shares, func(share infectious.Share) {
		copy(stripe[share.Number*s.ErasureShareSize():], share.Data)
	})
	if err != nil {
		return nil, err
	}

	return stripe, nil
}

func (s *unsafeRSScheme) ErasureShareSize() int {
	return s.erasureShareSize
}

func (s *unsafeRSScheme) StripeSize() int {
	return s.erasureShareSize * s.fc.Required()
}

func (s *unsafeRSScheme) TotalCount() int {
	return s.fc.Total()
}

func (s *unsafeRSScheme) RequiredCount() int {
	return s.fc.Required()
}
