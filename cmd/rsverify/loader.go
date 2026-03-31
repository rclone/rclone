package main

import (
	"fmt"
	"os"

	"github.com/rclone/rclone/backend/rs"
)

// loadShardsFromParticles reads particle files (any order), validates footers, and returns
// a shard slice indexed 0..k+m-1 (nil = missing).
func loadShardsFromParticles(paths []string) ([][]byte, int, int, int64, *rs.Footer, error) {
	var k, m int
	var contentLen int64
	var ref *rs.Footer
	var shards [][]byte
	first := true

	for _, p := range paths {
		raw, err := os.ReadFile(p)
		if err != nil {
			return nil, 0, 0, 0, nil, err
		}
		payload, ft, ok := tryParticleFooter(raw)
		if !ok {
			return nil, 0, 0, 0, nil, fmt.Errorf("%s: not a valid rclone EC particle (footer/CRC)", p)
		}
		if !algorithmIsRS(ft) {
			return nil, 0, 0, 0, nil, fmt.Errorf("%s: algorithm is not RS", p)
		}
		if first {
			k = int(ft.DataShards)
			m = int(ft.ParityShards)
			contentLen = ft.ContentLength
			shards = make([][]byte, k+m)
			ref = ft
			first = false
		} else if int(ft.DataShards) != k || int(ft.ParityShards) != m || ft.ContentLength != contentLen {
			return nil, 0, 0, 0, nil, fmt.Errorf("%s: footer metadata mismatch with other shards", p)
		}
		idx := int(ft.CurrentShard)
		if idx < 0 || idx >= k+m {
			return nil, 0, 0, 0, nil, fmt.Errorf("%s: invalid CurrentShard %d", p, idx)
		}
		if shards[idx] != nil {
			return nil, 0, 0, 0, nil, fmt.Errorf("%s: duplicate shard index %d", p, idx)
		}
		shards[idx] = payload
	}
	if first {
		return nil, 0, 0, 0, nil, fmt.Errorf("no shard files")
	}
	return shards, k, m, contentLen, ref, nil
}
