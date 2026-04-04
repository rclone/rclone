package main

import "fmt"

func countShardsOK(shards [][]byte, k int) error {
	n := 0
	for _, s := range shards {
		if s != nil {
			n++
		}
	}
	if n < k {
		return fmt.Errorf("need at least %d data shards (non-nil), have %d", k, n)
	}
	return nil
}
