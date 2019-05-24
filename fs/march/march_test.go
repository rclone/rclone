// Internal tests for march

package march

import (
	"fmt"
	"strings"
	"testing"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fstest/mockdir"
	"github.com/ncw/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
)

func TestNewMatchEntries(t *testing.T) {
	var (
		a = mockobject.Object("path/a")
		A = mockobject.Object("path/A")
		B = mockobject.Object("path/B")
		c = mockobject.Object("path/c")
	)

	es := newMatchEntries(fs.DirEntries{a, A, B, c}, nil)
	assert.Equal(t, es, matchEntries{
		{name: "A", leaf: "A", entry: A},
		{name: "B", leaf: "B", entry: B},
		{name: "a", leaf: "a", entry: a},
		{name: "c", leaf: "c", entry: c},
	})

	es = newMatchEntries(fs.DirEntries{a, A, B, c}, []matchTransformFn{strings.ToLower})
	assert.Equal(t, es, matchEntries{
		{name: "a", leaf: "A", entry: A},
		{name: "a", leaf: "a", entry: a},
		{name: "b", leaf: "B", entry: B},
		{name: "c", leaf: "c", entry: c},
	})
}

func TestMatchListings(t *testing.T) {
	var (
		a    = mockobject.Object("a")
		A    = mockobject.Object("A")
		b    = mockobject.Object("b")
		c    = mockobject.Object("c")
		d    = mockobject.Object("d")
		dirA = mockdir.New("A")
		dirb = mockdir.New("b")
	)

	for _, test := range []struct {
		what       string
		input      fs.DirEntries // pairs of input src, dst
		srcOnly    fs.DirEntries
		dstOnly    fs.DirEntries
		matches    []matchPair // pairs of output
		transforms []matchTransformFn
	}{
		{
			what: "only src or dst",
			input: fs.DirEntries{
				a, nil,
				b, nil,
				c, nil,
				d, nil,
			},
			srcOnly: fs.DirEntries{
				a, b, c, d,
			},
		},
		{
			what: "typical sync #1",
			input: fs.DirEntries{
				a, nil,
				b, b,
				nil, c,
				nil, d,
			},
			srcOnly: fs.DirEntries{
				a,
			},
			dstOnly: fs.DirEntries{
				c, d,
			},
			matches: []matchPair{
				{b, b},
			},
		},
		{
			what: "typical sync #2",
			input: fs.DirEntries{
				a, a,
				b, b,
				nil, c,
				d, d,
			},
			dstOnly: fs.DirEntries{
				c,
			},
			matches: []matchPair{
				{a, a},
				{b, b},
				{d, d},
			},
		},
		{
			what: "One duplicate",
			input: fs.DirEntries{
				A, A,
				a, a,
				a, nil,
				b, b,
			},
			matches: []matchPair{
				{A, A},
				{a, a},
				{b, b},
			},
		},
		{
			what: "Two duplicates",
			input: fs.DirEntries{
				a, a,
				a, a,
				a, nil,
			},
			matches: []matchPair{
				{a, a},
			},
		},
		{
			what: "Case insensitive duplicate - no transform",
			input: fs.DirEntries{
				a, a,
				A, A,
			},
			matches: []matchPair{
				{A, A},
				{a, a},
			},
		},
		{
			what: "Case insensitive duplicate - transform to lower case",
			input: fs.DirEntries{
				a, a,
				A, A,
			},
			matches: []matchPair{
				{A, A},
			},
			transforms: []matchTransformFn{strings.ToLower},
		},
		{
			what: "File and directory are not duplicates - srcOnly",
			input: fs.DirEntries{
				dirA, nil,
				A, nil,
			},
			srcOnly: fs.DirEntries{
				dirA,
				A,
			},
		},
		{
			what: "File and directory are not duplicates - matches",
			input: fs.DirEntries{
				dirA, dirA,
				A, A,
			},
			matches: []matchPair{
				{dirA, dirA},
				{A, A},
			},
		},
		{
			what: "Sync with directory #1",
			input: fs.DirEntries{
				dirA, nil,
				A, nil,
				b, b,
				nil, c,
				nil, d,
			},
			srcOnly: fs.DirEntries{
				dirA,
				A,
			},
			dstOnly: fs.DirEntries{
				c, d,
			},
			matches: []matchPair{
				{b, b},
			},
		},
		{
			what: "Sync with 2 directories",
			input: fs.DirEntries{
				dirA, dirA,
				A, nil,
				nil, dirb,
				nil, b,
			},
			srcOnly: fs.DirEntries{
				A,
			},
			dstOnly: fs.DirEntries{
				dirb,
				b,
			},
			matches: []matchPair{
				{dirA, dirA},
			},
		},
	} {
		t.Run(fmt.Sprintf("TestMatchListings-%s", test.what), func(t *testing.T) {
			var srcList, dstList fs.DirEntries
			for i := 0; i < len(test.input); i += 2 {
				src, dst := test.input[i], test.input[i+1]
				if src != nil {
					srcList = append(srcList, src)
				}
				if dst != nil {
					dstList = append(dstList, dst)
				}
			}
			srcOnly, dstOnly, matches := matchListings(srcList, dstList, test.transforms)
			assert.Equal(t, test.srcOnly, srcOnly, test.what, "srcOnly differ")
			assert.Equal(t, test.dstOnly, dstOnly, test.what, "dstOnly differ")
			assert.Equal(t, test.matches, matches, test.what, "matches differ")
			// now swap src and dst
			dstOnly, srcOnly, matches = matchListings(dstList, srcList, test.transforms)
			assert.Equal(t, test.srcOnly, srcOnly, test.what, "srcOnly differ")
			assert.Equal(t, test.dstOnly, dstOnly, test.what, "dstOnly differ")
			assert.Equal(t, test.matches, matches, test.what, "matches differ")
		})
	}
}
