package fs

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetadataSet(t *testing.T) {
	var m Metadata
	assert.Nil(t, m)
	m.Set("key", "value")
	assert.NotNil(t, m)
	assert.Equal(t, "value", m["key"])
	m.Set("key", "value2")
	assert.Equal(t, "value2", m["key"])
}

func TestMetadataMerge(t *testing.T) {
	for _, test := range []struct {
		in    Metadata
		merge Metadata
		want  Metadata
	}{
		{
			in:    Metadata{},
			merge: Metadata{},
			want:  Metadata{},
		}, {
			in:    nil,
			merge: nil,
			want:  nil,
		}, {
			in:    nil,
			merge: Metadata{},
			want:  nil,
		}, {
			in:    nil,
			merge: Metadata{"a": "1", "b": "2"},
			want:  Metadata{"a": "1", "b": "2"},
		}, {
			in:    Metadata{"a": "1", "b": "2"},
			merge: nil,
			want:  Metadata{"a": "1", "b": "2"},
		}, {
			in:    Metadata{"a": "1", "b": "2"},
			merge: Metadata{"b": "B", "c": "3"},
			want:  Metadata{"a": "1", "b": "B", "c": "3"},
		},
	} {
		what := fmt.Sprintf("in=%v, merge=%v", test.in, test.merge)
		test.in.Merge(test.merge)
		assert.Equal(t, test.want, test.in, what)
	}
}

func TestMetadataMergeOptions(t *testing.T) {
	for _, test := range []struct {
		in   Metadata
		opts []OpenOption
		want Metadata
	}{
		{
			opts: []OpenOption{},
			want: nil,
		}, {
			opts: []OpenOption{&HTTPOption{}},
			want: nil,
		}, {
			opts: []OpenOption{MetadataOption{"a": "1", "b": "2"}},
			want: Metadata{"a": "1", "b": "2"},
		}, {
			opts: []OpenOption{
				&HTTPOption{},
				MetadataOption{"a": "1", "b": "2"},
				MetadataOption{"b": "B", "c": "3"},
				&HTTPOption{},
			},
			want: Metadata{"a": "1", "b": "B", "c": "3"},
		}, {
			in: Metadata{"a": "first", "z": "OK"},
			opts: []OpenOption{
				&HTTPOption{},
				MetadataOption{"a": "1", "b": "2"},
				MetadataOption{"b": "B", "c": "3"},
				&HTTPOption{},
			},
			want: Metadata{"a": "1", "b": "B", "c": "3", "z": "OK"},
		},
	} {
		what := fmt.Sprintf("in=%v, opts=%v", test.in, test.opts)
		test.in.MergeOptions(test.opts)
		assert.Equal(t, test.want, test.in, what)
	}
}
