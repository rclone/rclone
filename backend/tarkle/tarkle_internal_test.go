package tarkle

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/tarkle/api"
	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartPlan(t *testing.T) {
	const mib = 1 << 20
	t.Run("single is one part of the whole file", func(t *testing.T) {
		p, err := newPartPlan(5*mib, 0, 0, true)
		require.NoError(t, err)
		assert.Equal(t, 1, p.count)
		off, n, ok := p.bounds(1)
		assert.True(t, ok)
		assert.Equal(t, int64(0), off)
		assert.Equal(t, int64(5*mib), n)
	})
	t.Run("last part is the remainder", func(t *testing.T) {
		p, err := newPartPlan(300*mib, 8*mib, 38, false)
		require.NoError(t, err)
		off, n, _ := p.bounds(38)
		assert.Equal(t, int64(37*8*mib), off)
		assert.Equal(t, int64(300*mib-37*8*mib), n)
		_, n, _ = p.bounds(1)
		assert.Equal(t, int64(8*mib), n)
	})
	t.Run("exact multiple has no empty last part", func(t *testing.T) {
		p, err := newPartPlan(16*mib, 8*mib, 2, false)
		require.NoError(t, err)
		_, n, _ := p.bounds(2)
		assert.Equal(t, int64(8*mib), n)
	})
	t.Run("out of range parts", func(t *testing.T) {
		p, _ := newPartPlan(16*mib, 8*mib, 2, false)
		for _, n := range []int{0, 3, -1} {
			_, _, ok := p.bounds(n)
			assert.False(t, ok, n)
		}
	})
	t.Run("a count that does not fit the size is refused", func(t *testing.T) {
		_, err := newPartPlan(16*mib, 8*mib, 3, false)
		assert.Error(t, err)
		_, err = newPartPlan(16*mib, 0, 2, false)
		assert.Error(t, err)
	})
	t.Run("200 GB with 10000 parts of 20 MB", func(t *testing.T) {
		const size = 200 << 30
		part := int64(21474837) // ceil(200 GiB / 10000)
		count := int((size + part - 1) / part)
		p, err := newPartPlan(size, part, count, false)
		require.NoError(t, err)
		var total int64
		for n := 1; n <= p.count; n++ {
			_, l, _ := p.bounds(n)
			total += l
		}
		assert.Equal(t, int64(size), total)
	})
}

func details(t *testing.T, v string) api.IncompleteDetails {
	var d api.IncompleteDetails
	require.NoError(t, json.Unmarshal([]byte(v), &d))
	return d
}

func TestMissingParts(t *testing.T) {
	m, ok := missingParts(details(t, `{"parts_received":[1,2,4,6],"parts_needed":6}`), 6)
	assert.True(t, ok)
	assert.Equal(t, []int{3, 5}, m)

	m, ok = missingParts(details(t, `{"missing_parts":[5,2]}`), 6)
	assert.True(t, ok)
	assert.Equal(t, []int{2, 5}, m)

	// counts alone can't tell which parts to send again
	_, ok = missingParts(details(t, `{"parts_received":4,"parts_needed":6}`), 6)
	assert.False(t, ok)

	m, ok = missingParts(details(t, `{"parts_received":[],"parts_needed":3}`), 3)
	assert.True(t, ok)
	assert.Equal(t, []int{1, 2, 3}, m)
}

func TestChoiceOffered(t *testing.T) {
	e := func(body string) *api.Error {
		x := &api.Error{}
		require.NoError(t, json.Unmarshal([]byte(body), x))
		return x
	}
	assert.True(t, choiceOffered(e(`{"error":{"code":"conflict","details":{"choices":["version","separate","skip"]}}}`), "version"))
	assert.False(t, choiceOffered(e(`{"error":{"code":"conflict","details":{"choices":["separate","skip"]}}}`), "version"))
	assert.True(t, choiceOffered(e(`{"error":{"code":"conflict"}}`), "version"))
}

func TestRelativePath(t *testing.T) {
	for _, tc := range []struct {
		prefix, p, want string
		ok              bool
	}{
		{"/", "/a/b.txt", "a/b.txt", true},
		{"/a", "/a/b/c.txt", "b/c.txt", true},
		{"/a", "/ab/c.txt", "", false},
		{"/a", "/a", "", false},
	} {
		got, ok := relativePath(tc.prefix, tc.p)
		assert.Equal(t, tc.ok, ok, tc)
		assert.Equal(t, tc.want, got, tc)
	}
}

func TestPatchJSON(t *testing.T) {
	name := "x"
	b, _ := json.Marshal(api.Patch{Name: &name})
	assert.JSONEq(t, `{"name":"x"}`, string(b))
	b, _ = json.Marshal(api.Patch{Move: true}) // move to the root
	assert.JSONEq(t, `{"parent_id":null}`, string(b))
	id := "abc"
	b, _ = json.Marshal(api.Patch{Move: true, ParentID: &id, Name: &name})
	assert.JSONEq(t, `{"name":"x","parent_id":"abc"}`, string(b))
	when := "2001-02-03T04:05:06.500Z"
	b, _ = json.Marshal(api.Patch{ModifiedAt: &when}) // mtime alone
	assert.JSONEq(t, `{"modified_at":"2001-02-03T04:05:06.500Z"}`, string(b))
}

func TestWalkListBatches(t *testing.T) {
	var batches []int
	w := walkList{max: 2, callback: func(e fs.DirEntries) error {
		batches = append(batches, len(e))
		return nil
	}}
	for i := 0; i < 5; i++ {
		require.NoError(t, w.add(fs.NewDir("d", time.Time{})))
	}
	require.NoError(t, w.flush())
	assert.Equal(t, []int{2, 2, 1}, batches)
}

func TestFormatTime(t *testing.T) {
	loc := time.FixedZone("x", 3600)
	assert.Equal(t, "2001-02-03T04:05:06.500Z", formatTime(time.Date(2001, 2, 3, 5, 5, 6, 500_999_999, loc)))
}
