// Copyright (c) 2020 MinIO Inc. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

package md5simd

import (
	"sort"
)

// Helper struct for sorting blocks based on length
type lane struct {
	len uint
	pos uint
}

// Helper struct for generating number of rounds in combination with mask for valid lanes
type maskRounds struct {
	mask   uint64
	rounds uint64
}

func generateMaskAndRounds8(input [8][]byte, mr *[8]maskRounds) (rounds int) {
	// Sort on blocks length small to large
	var sorted [8]lane
	for c, inpt := range input {
		sorted[c] = lane{uint(len(inpt)), uint(c)}
	}
	sort.Slice(sorted[:], func(i, j int) bool { return sorted[i].len < sorted[j].len })

	// Create mask array including 'rounds' (of processing blocks of 64 bytes) between masks
	m, round := uint64(0xff), uint64(0)

	for _, s := range sorted {
		if s.len > 0 {
			if uint64(s.len)>>6 > round {
				mr[rounds] = maskRounds{m, (uint64(s.len) >> 6) - round}
				rounds++
			}
			round = uint64(s.len) >> 6
		}
		m = m & ^(1 << uint(s.pos))
	}
	return
}

func generateMaskAndRounds16(input [16][]byte, mr *[16]maskRounds) (rounds int) {

	// Sort on blocks length small to large
	var sorted [16]lane
	for c, inpt := range input {
		sorted[c] = lane{uint(len(inpt)), uint(c)}
	}
	sort.Slice(sorted[:], func(i, j int) bool { return sorted[i].len < sorted[j].len })

	// Create mask array including 'rounds' (of processing blocks of 64 bytes) between masks
	m, round := uint64(0xffff), uint64(0)

	for _, s := range sorted {
		if s.len > 0 {
			if uint64(s.len)>>6 > round {
				mr[rounds] = maskRounds{m, (uint64(s.len) >> 6) - round}
				rounds++
			}
			round = uint64(s.len) >> 6
		}
		m = m & ^(1 << uint(s.pos))
	}
	return
}
