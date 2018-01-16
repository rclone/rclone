package bolt

import (
	"math/rand"
	"reflect"
	"sort"
	"testing"
	"unsafe"
)

// Ensure that a page is added to a transaction's freelist.
func TestFreelist_free(t *testing.T) {
	f := newFreelist()
	f.free(100, &page{id: 12})
	if !reflect.DeepEqual([]pgid{12}, f.pending[100].ids) {
		t.Fatalf("exp=%v; got=%v", []pgid{12}, f.pending[100])
	}
}

// Ensure that a page and its overflow is added to a transaction's freelist.
func TestFreelist_free_overflow(t *testing.T) {
	f := newFreelist()
	f.free(100, &page{id: 12, overflow: 3})
	if exp := []pgid{12, 13, 14, 15}; !reflect.DeepEqual(exp, f.pending[100].ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.pending[100])
	}
}

// Ensure that a transaction's free pages can be released.
func TestFreelist_release(t *testing.T) {
	f := newFreelist()
	f.free(100, &page{id: 12, overflow: 1})
	f.free(100, &page{id: 9})
	f.free(102, &page{id: 39})
	f.release(100)
	f.release(101)
	if exp := []pgid{9, 12, 13}; !reflect.DeepEqual(exp, f.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.ids)
	}

	f.release(102)
	if exp := []pgid{9, 12, 13, 39}; !reflect.DeepEqual(exp, f.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.ids)
	}
}

// Ensure that releaseRange handles boundary conditions correctly
func TestFreelist_releaseRange(t *testing.T) {
	type testRange struct {
		begin, end txid
	}

	type testPage struct {
		id       pgid
		n        int
		allocTxn txid
		freeTxn  txid
	}

	var releaseRangeTests = []struct {
		title         string
		pagesIn       []testPage
		releaseRanges []testRange
		wantFree      []pgid
	}{
		{
			title:         "Single pending in range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 300}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending with minimum end range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 200}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending outsize minimum end range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{1, 199}},
			wantFree:      nil,
		},
		{
			title:         "Single pending with minimum begin range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{100, 300}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending outside minimum begin range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 100, freeTxn: 200}},
			releaseRanges: []testRange{{101, 300}},
			wantFree:      nil,
		},
		{
			title:         "Single pending in minimum range",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 199, freeTxn: 200}},
			releaseRanges: []testRange{{199, 200}},
			wantFree:      []pgid{3},
		},
		{
			title:         "Single pending and read transaction at 199",
			pagesIn:       []testPage{{id: 3, n: 1, allocTxn: 199, freeTxn: 200}},
			releaseRanges: []testRange{{100, 198}, {200, 300}},
			wantFree:      nil,
		},
		{
			title: "Adjacent pending and read transactions at 199, 200",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 199, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 200, freeTxn: 201},
			},
			releaseRanges: []testRange{
				{100, 198},
				{200, 199}, // Simulate the ranges db.freePages might produce.
				{201, 300},
			},
			wantFree: nil,
		},
		{
			title: "Out of order ranges",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 199, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 200, freeTxn: 201},
			},
			releaseRanges: []testRange{
				{201, 199},
				{201, 200},
				{200, 200},
			},
			wantFree: nil,
		},
		{
			title: "Multiple pending, read transaction at 150",
			pagesIn: []testPage{
				{id: 3, n: 1, allocTxn: 100, freeTxn: 200},
				{id: 4, n: 1, allocTxn: 100, freeTxn: 125},
				{id: 5, n: 1, allocTxn: 125, freeTxn: 150},
				{id: 6, n: 1, allocTxn: 125, freeTxn: 175},
				{id: 7, n: 2, allocTxn: 150, freeTxn: 175},
				{id: 9, n: 2, allocTxn: 175, freeTxn: 200},
			},
			releaseRanges: []testRange{{50, 149}, {151, 300}},
			wantFree:      []pgid{4, 9},
		},
	}

	for _, c := range releaseRangeTests {
		f := newFreelist()

		for _, p := range c.pagesIn {
			for i := uint64(0); i < uint64(p.n); i++ {
				f.ids = append(f.ids, pgid(uint64(p.id)+i))
			}
		}
		for _, p := range c.pagesIn {
			f.allocate(p.allocTxn, p.n)
		}

		for _, p := range c.pagesIn {
			f.free(p.freeTxn, &page{id: p.id})
		}

		for _, r := range c.releaseRanges {
			f.releaseRange(r.begin, r.end)
		}

		if exp := c.wantFree; !reflect.DeepEqual(exp, f.ids) {
			t.Errorf("exp=%v; got=%v for %s", exp, f.ids, c.title)
		}
	}
}

// Ensure that a freelist can find contiguous blocks of pages.
func TestFreelist_allocate(t *testing.T) {
	f := newFreelist()
	f.ids = []pgid{3, 4, 5, 6, 7, 9, 12, 13, 18}
	if id := int(f.allocate(1, 3)); id != 3 {
		t.Fatalf("exp=3; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 6 {
		t.Fatalf("exp=6; got=%v", id)
	}
	if id := int(f.allocate(1, 3)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if id := int(f.allocate(1, 2)); id != 12 {
		t.Fatalf("exp=12; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 7 {
		t.Fatalf("exp=7; got=%v", id)
	}
	if id := int(f.allocate(1, 0)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if id := int(f.allocate(1, 0)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if exp := []pgid{9, 18}; !reflect.DeepEqual(exp, f.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.ids)
	}

	if id := int(f.allocate(1, 1)); id != 9 {
		t.Fatalf("exp=9; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 18 {
		t.Fatalf("exp=18; got=%v", id)
	}
	if id := int(f.allocate(1, 1)); id != 0 {
		t.Fatalf("exp=0; got=%v", id)
	}
	if exp := []pgid{}; !reflect.DeepEqual(exp, f.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.ids)
	}
}

// Ensure that a freelist can deserialize from a freelist page.
func TestFreelist_read(t *testing.T) {
	// Create a page.
	var buf [4096]byte
	page := (*page)(unsafe.Pointer(&buf[0]))
	page.flags = freelistPageFlag
	page.count = 2

	// Insert 2 page ids.
	ids := (*[3]pgid)(unsafe.Pointer(&page.ptr))
	ids[0] = 23
	ids[1] = 50

	// Deserialize page into a freelist.
	f := newFreelist()
	f.read(page)

	// Ensure that there are two page ids in the freelist.
	if exp := []pgid{23, 50}; !reflect.DeepEqual(exp, f.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f.ids)
	}
}

// Ensure that a freelist can serialize into a freelist page.
func TestFreelist_write(t *testing.T) {
	// Create a freelist and write it to a page.
	var buf [4096]byte
	f := &freelist{ids: []pgid{12, 39}, pending: make(map[txid]*txPending)}
	f.pending[100] = &txPending{ids: []pgid{28, 11}}
	f.pending[101] = &txPending{ids: []pgid{3}}
	p := (*page)(unsafe.Pointer(&buf[0]))
	if err := f.write(p); err != nil {
		t.Fatal(err)
	}

	// Read the page back out.
	f2 := newFreelist()
	f2.read(p)

	// Ensure that the freelist is correct.
	// All pages should be present and in reverse order.
	if exp := []pgid{3, 11, 12, 28, 39}; !reflect.DeepEqual(exp, f2.ids) {
		t.Fatalf("exp=%v; got=%v", exp, f2.ids)
	}
}

func Benchmark_FreelistRelease10K(b *testing.B)    { benchmark_FreelistRelease(b, 10000) }
func Benchmark_FreelistRelease100K(b *testing.B)   { benchmark_FreelistRelease(b, 100000) }
func Benchmark_FreelistRelease1000K(b *testing.B)  { benchmark_FreelistRelease(b, 1000000) }
func Benchmark_FreelistRelease10000K(b *testing.B) { benchmark_FreelistRelease(b, 10000000) }

func benchmark_FreelistRelease(b *testing.B, size int) {
	ids := randomPgids(size)
	pending := randomPgids(len(ids) / 400)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		txp := &txPending{ids: pending}
		f := &freelist{ids: ids, pending: map[txid]*txPending{1: txp}}
		f.release(1)
	}
}

func randomPgids(n int) []pgid {
	rand.Seed(42)
	pgids := make(pgids, n)
	for i := range pgids {
		pgids[i] = pgid(rand.Int63())
	}
	sort.Sort(pgids)
	return pgids
}
