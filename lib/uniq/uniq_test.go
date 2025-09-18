package uniq

import (
	"sync"
	"testing"
)

func TestNextUniqueness(t *testing.T) {
	const total = 1024
	g := New("test-seed")
	seen := make(map[int64]struct{}, total)
	for i := 0; i < total; i++ {
		id := g.Next()
		if id < 0 {
			t.Fatalf("expected non-negative id, got %d", id)
		}
		if id > int64(jsMaxInt) {
			t.Fatalf("id %d exceeds js max int", id)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id generated: %d", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNextConcurrentUniqueness(t *testing.T) {
	const total = 4096
	g := New("concurrency-seed")
	results := make(chan int64, total)

	var wg sync.WaitGroup
	goroutines := 32
	base := total / goroutines
	remainder := total % goroutines
	for i := 0; i < goroutines; i++ {
		count := base
		if i < remainder {
			count++
		}
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < n; j++ {
				results <- g.Next()
			}
		}(count)
	}

	wg.Wait()
	close(results)

	seen := make(map[int64]struct{}, total)
	for id := range results {
		if id < 0 {
			t.Fatalf("expected non-negative id, got %d", id)
		}
		if id > int64(jsMaxInt) {
			t.Fatalf("id %d exceeds js max int", id)
		}
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id generated: %d", id)
		}
		seen[id] = struct{}{}
	}
}

func TestNextPrefixConsistency(t *testing.T) {
	g := New("prefix-seed")
	first := g.Next()
	firstPrefix := int64(uint64(first) >> counterBits)
	if firstPrefix != int64(g.prefix) {
		t.Fatalf("expected prefix %d, got %d", g.prefix, firstPrefix)
	}
	for i := 0; i < 10; i++ {
		id := g.Next()
		if int64(uint64(id)>>counterBits) != firstPrefix {
			t.Fatalf("id %d does not share prefix %d", id, firstPrefix)
		}
	}
}

func TestNewEmptySeed(t *testing.T) {
	g := New("")
	if g.prefix == 0 {
		t.Fatal("expected non-zero prefix for empty seed")
	}
	id := g.Next()
	if id <= 0 {
		t.Fatalf("expected positive id, got %d", id)
	}
}