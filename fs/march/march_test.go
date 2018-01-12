// Internal tests for march

package march

import (
	"strings"
	"testing"

	"github.com/ncw/rclone/fs"
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
		a = mockobject.Object("a")
		A = mockobject.Object("A")
		b = mockobject.Object("b")
		c = mockobject.Object("c")
		d = mockobject.Object("d")
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
	} {
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
		assert.Equal(t, test.srcOnly, srcOnly, test.what)
		assert.Equal(t, test.dstOnly, dstOnly, test.what)
		assert.Equal(t, test.matches, matches, test.what)
		// now swap src and dst
		dstOnly, srcOnly, matches = matchListings(dstList, srcList, test.transforms)
		assert.Equal(t, test.srcOnly, srcOnly, test.what)
		assert.Equal(t, test.dstOnly, dstOnly, test.what)
		assert.Equal(t, test.matches, matches, test.what)
	}
}
