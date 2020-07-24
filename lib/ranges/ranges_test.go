package ranges

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRangeEnd(t *testing.T) {
	assert.Equal(t, int64(3), Range{Pos: 1, Size: 2}.End())
}

func TestRangeIsEmpty(t *testing.T) {
	assert.Equal(t, false, Range{Pos: 1, Size: 2}.IsEmpty())
	assert.Equal(t, true, Range{Pos: 1, Size: 0}.IsEmpty())
	assert.Equal(t, true, Range{Pos: 1, Size: -1}.IsEmpty())
}

func TestRangeClip(t *testing.T) {
	r := Range{Pos: 1, Size: 2}
	r.Clip(5)
	assert.Equal(t, Range{Pos: 1, Size: 2}, r)

	r = Range{Pos: 1, Size: 6}
	r.Clip(5)
	assert.Equal(t, Range{Pos: 1, Size: 4}, r)

	r = Range{Pos: 5, Size: 6}
	r.Clip(5)
	assert.Equal(t, Range{Pos: 5, Size: 0}, r)

	r = Range{Pos: 7, Size: 6}
	r.Clip(5)
	assert.Equal(t, Range{Pos: 0, Size: 0}, r)
}

func TestRangeIntersection(t *testing.T) {
	for _, test := range []struct {
		r    Range
		b    Range
		want Range
	}{
		{
			r:    Range{1, 1},
			b:    Range{3, 1},
			want: Range{},
		},
		{
			r:    Range{1, 1},
			b:    Range{1, 1},
			want: Range{1, 1},
		},
		{
			r:    Range{1, 9},
			b:    Range{3, 2},
			want: Range{3, 2},
		},
		{
			r:    Range{1, 5},
			b:    Range{3, 5},
			want: Range{3, 3},
		},
	} {
		what := fmt.Sprintf("test r=%v, b=%v", test.r, test.b)
		got := test.r.Intersection(test.b)
		assert.Equal(t, test.want, got, what)
		got = test.b.Intersection(test.r)
		assert.Equal(t, test.want, got, what)
	}
}

func TestRangeMerge(t *testing.T) {
	for _, test := range []struct {
		new        Range
		dst        Range
		want       Range
		wantMerged bool
	}{
		{
			new:        Range{Pos: 1, Size: 1}, // .N.......
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 3, Size: 3}, // ...DDD...
			wantMerged: false,
		},
		{
			new:        Range{Pos: 1, Size: 2}, // .NN......
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 1, Size: 5}, // .XXXXX...
			wantMerged: true,
		},
		{
			new:        Range{Pos: 1, Size: 3}, // .NNN.....
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 1, Size: 5}, // .XXXXX...
			wantMerged: true,
		},
		{
			new:        Range{Pos: 1, Size: 5}, // .NNNNN...
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 1, Size: 5}, // .XXXXX...
			wantMerged: true,
		},
		{
			new:        Range{Pos: 1, Size: 6}, // .NNNNNN..
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 1, Size: 6}, // .XXXXXX..
			wantMerged: true,
		},
		{
			new:        Range{Pos: 3, Size: 3}, // ...NNN...
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 3, Size: 3}, // ...XXX...
			wantMerged: true,
		},
		{
			new:        Range{Pos: 3, Size: 2}, // ...NN....
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 3, Size: 3}, // ...XXX...
			wantMerged: true,
		},
		{
			new:        Range{Pos: 3, Size: 4}, // ...NNNN..
			dst:        Range{Pos: 3, Size: 3}, // ...DDD...
			want:       Range{Pos: 3, Size: 4}, // ...XXXX..
			wantMerged: true,
		},
	} {
		what := fmt.Sprintf("test new=%v, dst=%v", test.new, test.dst)
		gotMerged := merge(&test.new, &test.dst)
		assert.Equal(t, test.wantMerged, gotMerged)
		assert.Equal(t, test.want, test.dst, what)
	}
}

func checkRanges(t *testing.T, rs Ranges, what string) bool {
	if len(rs) < 2 {
		return true
	}
	ok := true
	for i := 0; i < len(rs)-1; i++ {
		a := rs[i]
		b := rs[i+1]
		if a.Pos >= b.Pos {
			assert.Failf(t, "%s: Ranges in wrong order at %d in: %v", what, i, rs)
			ok = false
		}
		if a.End() > b.Pos {
			assert.Failf(t, "%s: Ranges overlap at %d in: %v", what, i, rs)
			ok = false
		}
		if a.End() == b.Pos {
			assert.Failf(t, "%s: Ranges not coalesced at %d in: %v", what, i, rs)
			ok = false
		}
	}
	return ok
}

func TestRangeCoalesce(t *testing.T) {
	for _, test := range []struct {
		rs   Ranges
		i    int
		want Ranges
	}{
		{
			rs:   Ranges{},
			want: Ranges{},
		},
		{
			rs: Ranges{
				{Pos: 1, Size: 1},
			},
			want: Ranges{
				{Pos: 1, Size: 1},
			},
			i: 0,
		},
		{
			rs: Ranges{
				{Pos: 1, Size: 1},
				{Pos: 2, Size: 1},
				{Pos: 3, Size: 1},
			},
			want: Ranges{
				{Pos: 1, Size: 3},
			},
			i: 0,
		},
		{
			rs: Ranges{
				{Pos: 1, Size: 1},
				{Pos: 3, Size: 1},
				{Pos: 4, Size: 1},
				{Pos: 5, Size: 1},
			},
			want: Ranges{
				{Pos: 1, Size: 1},
				{Pos: 3, Size: 3},
			},
			i: 2,
		},
		{
			rs:   Ranges{{38, 8}, {51, 10}, {60, 3}},
			want: Ranges{{38, 8}, {51, 12}},
			i:    1,
		},
	} {
		got := append(Ranges{}, test.rs...)
		got.coalesce(test.i)
		what := fmt.Sprintf("test rs=%v, i=%d", test.rs, test.i)
		assert.Equal(t, test.want, got, what)
		checkRanges(t, got, what)
	}
}

func TestRangeInsert(t *testing.T) {
	for _, test := range []struct {
		new  Range
		rs   Ranges
		want Ranges
	}{
		{
			new:  Range{Pos: 1, Size: 0},
			rs:   Ranges{},
			want: Ranges(nil),
		},
		{
			new: Range{Pos: 1, Size: 1}, // .N.......
			rs:  Ranges{},               // .........
			want: Ranges{ // .N.......
				{Pos: 1, Size: 1},
			},
		},
		{
			new: Range{Pos: 1, Size: 1},    // .N.......
			rs:  Ranges{{Pos: 5, Size: 1}}, // .....R...
			want: Ranges{ // .N...R...
				{Pos: 1, Size: 1},
				{Pos: 5, Size: 1},
			},
		},
		{
			new: Range{Pos: 5, Size: 1},    // .....R...
			rs:  Ranges{{Pos: 1, Size: 1}}, // .N.......
			want: Ranges{ // .N...R...
				{Pos: 1, Size: 1},
				{Pos: 5, Size: 1},
			},
		},
		{
			new: Range{Pos: 1, Size: 1},    // .N.......
			rs:  Ranges{{Pos: 2, Size: 1}}, // ..R......
			want: Ranges{ // .XX......
				{Pos: 1, Size: 2},
			},
		},
		{
			new: Range{Pos: 2, Size: 1},    // ..N.......
			rs:  Ranges{{Pos: 1, Size: 1}}, // .R......
			want: Ranges{ // .XX......
				{Pos: 1, Size: 2},
			},
		},
		{
			new:  Range{Pos: 51, Size: 10},
			rs:   Ranges{{38, 8}, {57, 2}, {60, 3}},
			want: Ranges{{38, 8}, {51, 12}},
		},
	} {
		got := append(Ranges(nil), test.rs...)
		got.Insert(test.new)
		what := fmt.Sprintf("test new=%v, rs=%v", test.new, test.rs)
		assert.Equal(t, test.want, got, what)
		checkRanges(t, test.rs, what)
		checkRanges(t, got, what)
	}
}

func TestRangeInsertRandom(t *testing.T) {
	for i := 0; i < 100; i++ {
		var rs Ranges
		for j := 0; j < 100; j++ {
			var r = Range{
				Pos:  rand.Int63n(100),
				Size: rand.Int63n(10) + 1,
			}
			what := fmt.Sprintf("inserting %v into %v\n", r, rs)
			rs.Insert(r)
			if !checkRanges(t, rs, what) {
				break
			}
			//fmt.Printf("%d: %d: %v\n", i, j, rs)
		}
	}
}

func TestRangeFind(t *testing.T) {
	for _, test := range []struct {
		rs          Ranges
		r           Range
		wantCurr    Range
		wantNext    Range
		wantPresent bool
	}{
		{
			r:           Range{Pos: 1, Size: 0},
			rs:          Ranges{},
			wantCurr:    Range{Pos: 1, Size: 0},
			wantNext:    Range{},
			wantPresent: false,
		},
		{
			r:           Range{Pos: 1, Size: 1},
			rs:          Ranges{},
			wantCurr:    Range{Pos: 1, Size: 1},
			wantNext:    Range{},
			wantPresent: false,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 1, Size: 10},
			},
			wantCurr:    Range{Pos: 1, Size: 2},
			wantNext:    Range{Pos: 3, Size: 0},
			wantPresent: true,
		},
		{
			r: Range{Pos: 1, Size: 10},
			rs: Ranges{
				Range{Pos: 1, Size: 2},
			},
			wantCurr:    Range{Pos: 1, Size: 2},
			wantNext:    Range{Pos: 3, Size: 8},
			wantPresent: true,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 5, Size: 2},
			},
			wantCurr:    Range{Pos: 1, Size: 2},
			wantNext:    Range{Pos: 0, Size: 0},
			wantPresent: false,
		},
		{
			r: Range{Pos: 2, Size: 10},
			rs: Ranges{
				Range{Pos: 1, Size: 2},
			},
			wantCurr:    Range{Pos: 2, Size: 1},
			wantNext:    Range{Pos: 3, Size: 9},
			wantPresent: true,
		},
		{
			r: Range{Pos: 1, Size: 9},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantCurr:    Range{Pos: 1, Size: 1},
			wantNext:    Range{Pos: 2, Size: 8},
			wantPresent: false,
		},
		{
			r: Range{Pos: 2, Size: 8},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantCurr:    Range{Pos: 2, Size: 1},
			wantNext:    Range{Pos: 3, Size: 7},
			wantPresent: true,
		},
		{
			r: Range{Pos: 3, Size: 7},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantCurr:    Range{Pos: 3, Size: 1},
			wantNext:    Range{Pos: 4, Size: 6},
			wantPresent: false,
		},
		{
			r: Range{Pos: 4, Size: 6},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantCurr:    Range{Pos: 4, Size: 1},
			wantNext:    Range{Pos: 5, Size: 5},
			wantPresent: true,
		},
		{
			r: Range{Pos: 5, Size: 5},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantCurr:    Range{Pos: 5, Size: 5},
			wantNext:    Range{Pos: 0, Size: 0},
			wantPresent: false,
		},
	} {
		what := fmt.Sprintf("test r=%v, rs=%v", test.r, test.rs)
		checkRanges(t, test.rs, what)
		gotCurr, gotNext, gotPresent := test.rs.Find(test.r)
		assert.Equal(t, test.r.Pos, gotCurr.Pos, what)
		assert.Equal(t, test.wantCurr, gotCurr, what)
		assert.Equal(t, test.wantNext, gotNext, what)
		assert.Equal(t, test.wantPresent, gotPresent, what)
	}
}

func TestRangeFindAll(t *testing.T) {
	for _, test := range []struct {
		rs          Ranges
		r           Range
		want        []FoundRange
		wantNext    Range
		wantPresent bool
	}{
		{
			r:    Range{Pos: 1, Size: 0},
			rs:   Ranges{},
			want: []FoundRange(nil),
		},
		{
			r:  Range{Pos: 1, Size: 1},
			rs: Ranges{},
			want: []FoundRange{
				{
					R:       Range{Pos: 1, Size: 1},
					Present: false,
				},
			},
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 1, Size: 10},
			},
			want: []FoundRange{
				{
					R:       Range{Pos: 1, Size: 2},
					Present: true,
				},
			},
		},
		{
			r: Range{Pos: 1, Size: 10},
			rs: Ranges{
				Range{Pos: 1, Size: 2},
			},
			want: []FoundRange{
				{
					R:       Range{Pos: 1, Size: 2},
					Present: true,
				},
				{
					R:       Range{Pos: 3, Size: 8},
					Present: false,
				},
			},
		},
		{
			r: Range{Pos: 5, Size: 5},
			rs: Ranges{
				Range{Pos: 4, Size: 2},
				Range{Pos: 7, Size: 1},
				Range{Pos: 9, Size: 2},
			},
			want: []FoundRange{
				{
					R:       Range{Pos: 5, Size: 1},
					Present: true,
				},
				{
					R:       Range{Pos: 6, Size: 1},
					Present: false,
				},
				{
					R:       Range{Pos: 7, Size: 1},
					Present: true,
				},
				{
					R:       Range{Pos: 8, Size: 1},
					Present: false,
				},
				{
					R:       Range{Pos: 9, Size: 1},
					Present: true,
				},
			},
		},
	} {
		what := fmt.Sprintf("test r=%v, rs=%v", test.r, test.rs)
		checkRanges(t, test.rs, what)
		got := test.rs.FindAll(test.r)
		assert.Equal(t, test.want, got, what)
	}
}

func TestRangePresent(t *testing.T) {
	for _, test := range []struct {
		rs          Ranges
		r           Range
		wantPresent bool
	}{
		{
			r:           Range{Pos: 1, Size: 0},
			rs:          Ranges{},
			wantPresent: true,
		},
		{
			r:           Range{Pos: 1, Size: 0},
			rs:          Ranges(nil),
			wantPresent: true,
		},
		{
			r:           Range{Pos: 0, Size: 1},
			rs:          Ranges{},
			wantPresent: false,
		},
		{
			r:           Range{Pos: 0, Size: 1},
			rs:          Ranges(nil),
			wantPresent: false,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 1, Size: 1},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 1, Size: 2},
			},
			wantPresent: true,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 1, Size: 10},
			},
			wantPresent: true,
		},
		{
			r: Range{Pos: 1, Size: 2},
			rs: Ranges{
				Range{Pos: 5, Size: 2},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 1, Size: 9},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 2, Size: 8},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 3, Size: 7},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 4, Size: 6},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantPresent: false,
		},
		{
			r: Range{Pos: 5, Size: 5},
			rs: Ranges{
				Range{Pos: 2, Size: 1},
				Range{Pos: 4, Size: 1},
			},
			wantPresent: false,
		},
	} {
		what := fmt.Sprintf("test r=%v, rs=%v", test.r, test.rs)
		checkRanges(t, test.rs, what)
		gotPresent := test.rs.Present(test.r)
		assert.Equal(t, test.wantPresent, gotPresent, what)
		checkRanges(t, test.rs, what)
	}
}

func TestRangesIntersection(t *testing.T) {
	for _, test := range []struct {
		rs   Ranges
		r    Range
		want Ranges
	}{
		{
			rs:   Ranges(nil),
			r:    Range{},
			want: Ranges(nil),
		},
		{
			rs:   Ranges{},
			r:    Range{},
			want: Ranges{},
		},
		{
			rs:   Ranges{},
			r:    Range{Pos: 1, Size: 0},
			want: Ranges{},
		},
		{
			rs:   Ranges{},
			r:    Range{Pos: 1, Size: 1},
			want: Ranges{},
		},
		{
			rs: Ranges{{Pos: 1, Size: 5}},
			r:  Range{Pos: 1, Size: 3},
			want: Ranges{
				{Pos: 1, Size: 3},
			},
		},
		{
			rs: Ranges{{Pos: 1, Size: 5}},
			r:  Range{Pos: 1, Size: 10},
			want: Ranges{
				{Pos: 1, Size: 5},
			},
		},
		{
			rs: Ranges{{Pos: 1, Size: 5}},
			r:  Range{Pos: 3, Size: 10},
			want: Ranges{
				{Pos: 3, Size: 3},
			},
		},
		{
			rs:   Ranges{{Pos: 1, Size: 5}},
			r:    Range{Pos: 6, Size: 10},
			want: Ranges(nil),
		},
		{
			rs: Ranges{
				{Pos: 1, Size: 2},
				{Pos: 11, Size: 2},
				{Pos: 21, Size: 2},
				{Pos: 31, Size: 2},
				{Pos: 41, Size: 2},
			},
			r: Range{Pos: 12, Size: 20},
			want: Ranges{
				{Pos: 12, Size: 1},
				{Pos: 21, Size: 2},
				{Pos: 31, Size: 1},
			},
		},
	} {
		got := test.rs.Intersection(test.r)
		what := fmt.Sprintf("test ra=%v, r=%v", test.rs, test.r)
		assert.Equal(t, test.want, got, what)
		checkRanges(t, test.rs, what)
		checkRanges(t, got, what)
	}
}

func TestRangesEqual(t *testing.T) {
	for _, test := range []struct {
		rs   Ranges
		bs   Ranges
		want bool
	}{
		{
			rs:   Ranges(nil),
			bs:   Ranges(nil),
			want: true,
		},
		{
			rs:   Ranges{},
			bs:   Ranges(nil),
			want: true,
		},
		{
			rs:   Ranges(nil),
			bs:   Ranges{},
			want: true,
		},
		{
			rs:   Ranges{},
			bs:   Ranges{},
			want: true,
		},
		{
			rs: Ranges{
				{Pos: 0, Size: 1},
			},
			bs:   Ranges{},
			want: false,
		},
		{
			rs: Ranges{
				{Pos: 0, Size: 1},
			},
			bs: Ranges{
				{Pos: 0, Size: 1},
			},
			want: true,
		},
		{
			rs: Ranges{
				{Pos: 0, Size: 1},
				{Pos: 10, Size: 9},
				{Pos: 20, Size: 21},
			},
			bs: Ranges{
				{Pos: 0, Size: 1},
				{Pos: 10, Size: 9},
				{Pos: 20, Size: 22},
			},
			want: false,
		},
		{
			rs: Ranges{
				{Pos: 0, Size: 1},
				{Pos: 10, Size: 9},
				{Pos: 20, Size: 21},
			},
			bs: Ranges{
				{Pos: 0, Size: 1},
				{Pos: 10, Size: 9},
				{Pos: 20, Size: 21},
			},
			want: true,
		},
	} {
		got := test.rs.Equal(test.bs)
		what := fmt.Sprintf("test rs=%v, bs=%v", test.rs, test.bs)
		assert.Equal(t, test.want, got, what)
		checkRanges(t, test.bs, what)
		checkRanges(t, test.rs, what)
	}
}

func TestRangesSize(t *testing.T) {
	for _, test := range []struct {
		rs   Ranges
		want int64
	}{
		{
			rs:   Ranges(nil),
			want: 0,
		},
		{
			rs:   Ranges{},
			want: 0,
		},
		{
			rs: Ranges{
				{Pos: 7, Size: 11},
			},
			want: 11,
		},
		{
			rs: Ranges{
				{Pos: 0, Size: 1},
				{Pos: 10, Size: 9},
				{Pos: 20, Size: 21},
			},
			want: 31,
		},
	} {
		got := test.rs.Size()
		what := fmt.Sprintf("test rs=%v", test.rs)
		assert.Equal(t, test.want, got, what)
		checkRanges(t, test.rs, what)
	}
}

func TestFindMissing(t *testing.T) {
	for _, test := range []struct {
		r    Range
		rs   Ranges
		want Range
	}{
		{
			r:    Range{},
			rs:   Ranges(nil),
			want: Range{},
		},
		{
			r:    Range{},
			rs:   Ranges{},
			want: Range{},
		},
		{
			r: Range{Pos: 3, Size: 5},
			rs: Ranges{
				{Pos: 10, Size: 5},
				{Pos: 20, Size: 5},
			},
			want: Range{Pos: 3, Size: 5},
		},
		{
			r: Range{Pos: 3, Size: 15},
			rs: Ranges{
				{Pos: 10, Size: 5},
				{Pos: 20, Size: 5},
			},
			want: Range{Pos: 3, Size: 15},
		},
		{
			r: Range{Pos: 10, Size: 5},
			rs: Ranges{
				{Pos: 10, Size: 5},
				{Pos: 20, Size: 5},
			},
			want: Range{Pos: 15, Size: 0},
		},
		{
			r: Range{Pos: 10, Size: 7},
			rs: Ranges{
				{Pos: 10, Size: 5},
				{Pos: 20, Size: 5},
			},
			want: Range{Pos: 15, Size: 2},
		},
		{
			r: Range{Pos: 11, Size: 7},
			rs: Ranges{
				{Pos: 10, Size: 5},
				{Pos: 20, Size: 5},
			},
			want: Range{Pos: 15, Size: 3},
		},
	} {
		got := test.rs.FindMissing(test.r)
		what := fmt.Sprintf("test r=%v, rs=%v", test.r, test.rs)
		assert.Equal(t, test.want, got, what)
		assert.Equal(t, test.r.End(), got.End())
		checkRanges(t, test.rs, what)
	}
}
