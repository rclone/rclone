// Internal tests for sync/copy/move

package fs

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchListings(t *testing.T) {
	var (
		a = mockObject("a")
		b = mockObject("b")
		c = mockObject("c")
		d = mockObject("d")
	)

	for _, test := range []struct {
		what    string
		input   DirEntries // pairs of input src, dst
		srcOnly DirEntries
		dstOnly DirEntries
		matches []matchPair // pairs of output
	}{
		{
			what: "only src or dst",
			input: DirEntries{
				a, nil,
				b, nil,
				c, nil,
				d, nil,
			},
			srcOnly: DirEntries{
				a, b, c, d,
			},
		},
		{
			what: "typical sync #1",
			input: DirEntries{
				a, nil,
				b, b,
				nil, c,
				nil, d,
			},
			srcOnly: DirEntries{
				a,
			},
			dstOnly: DirEntries{
				c, d,
			},
			matches: []matchPair{
				{b, b},
			},
		},
		{
			what: "typical sync #2",
			input: DirEntries{
				a, a,
				b, b,
				nil, c,
				d, d,
			},
			dstOnly: DirEntries{
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
			input: DirEntries{
				a, a,
				a, nil,
			},
			matches: []matchPair{
				{a, a},
			},
		},
		{
			what: "Two duplicates",
			input: DirEntries{
				a, a,
				a, a,
				a, nil,
			},
			matches: []matchPair{
				{a, a},
			},
		},
		{
			what: "Out of order",
			input: DirEntries{
				c, nil,
				b, b,
				a, nil,
			},
			srcOnly: DirEntries{
				c,
			},
			dstOnly: DirEntries{
				b,
			},
		},
	} {
		var srcList, dstList DirEntries
		for i := 0; i < len(test.input); i += 2 {
			src, dst := test.input[i], test.input[i+1]
			if src != nil {
				srcList = append(srcList, src)
			}
			if dst != nil {
				dstList = append(dstList, dst)
			}
		}
		srcOnly, dstOnly, matches := matchListings(srcList, dstList)
		assert.Equal(t, test.srcOnly, srcOnly, test.what)
		assert.Equal(t, test.dstOnly, dstOnly, test.what)
		assert.Equal(t, test.matches, matches, test.what)
		// now swap src and dst
		dstOnly, srcOnly, matches = matchListings(dstList, srcList)
		assert.Equal(t, test.srcOnly, srcOnly, test.what)
		assert.Equal(t, test.dstOnly, dstOnly, test.what)
		assert.Equal(t, test.matches, matches, test.what)
	}
}
