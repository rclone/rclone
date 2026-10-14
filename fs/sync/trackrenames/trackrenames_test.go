package trackrenames_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/sync/trackrenames"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseStrategy(t *testing.T) {
	tests := []struct {
		name    string
		value   string
		want    trackrenames.Strategy
		wantErr string
	}{
		{name: "empty"},
		{name: "size", value: "size"},
		{name: "hash", value: "hash", want: trackrenames.StrategyHash},
		{name: "modtime", value: "modtime", want: trackrenames.StrategyModtime},
		{name: "leaf", value: "leaf", want: trackrenames.StrategyLeaf},
		{
			name:  "combined",
			value: "hash,modtime,size,leaf",
			want:  trackrenames.StrategyHash | trackrenames.StrategyModtime | trackrenames.StrategyLeaf,
		},
		{name: "unknown", value: "size,unknown", wantErr: `unknown track renames strategy "unknown"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := trackrenames.ParseStrategy(test.value)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestStrategyUses(t *testing.T) {
	strategy := trackrenames.StrategyHash | trackrenames.StrategyLeaf
	assert.True(t, strategy.UsesHash())
	assert.False(t, strategy.UsesModtime())
	assert.True(t, strategy.UsesLeaf())
}

func TestMatcher(t *testing.T) {
	ctx := context.Background()
	baseTime := time.Date(2026, time.August, 24, 12, 0, 0, 0, time.UTC)
	newObject := func(remote, contents string, modTime time.Time) *object.MemoryObject {
		return object.NewMemoryObject(remote, modTime, []byte(contents))
	}

	t.Run("size match is consumed", func(t *testing.T) {
		dst := newObject("old/file.txt", "same size", baseTime)
		src := newObject("new/file.txt", "different", baseTime)
		matcher := trackrenames.NewMatcher(ctx, 0, hash.None, 0)

		assert.True(t, matcher.Add(dst))
		assert.Same(t, dst, matcher.Match(src))
		assert.Nil(t, matcher.Match(src))
	})

	t.Run("hash rejects different content", func(t *testing.T) {
		dst := newObject("old/file.txt", "destination", baseTime)
		src := newObject("new/file.txt", "source_____", baseTime)
		matcher := trackrenames.NewMatcher(ctx, trackrenames.StrategyHash, hash.MD5, 0)

		assert.True(t, matcher.Add(dst))
		assert.Nil(t, matcher.Match(src))
	})

	t.Run("leaf matches basename", func(t *testing.T) {
		dst := newObject("old/file.txt", "contents", baseTime)
		sameLeaf := newObject("new/file.txt", "contents", baseTime)
		differentLeaf := newObject("new/other.txt", "contents", baseTime)
		matcher := trackrenames.NewMatcher(ctx, trackrenames.StrategyLeaf, hash.None, 0)

		assert.True(t, matcher.Add(dst))
		assert.Nil(t, matcher.Match(differentLeaf))
		assert.Same(t, dst, matcher.Match(sameLeaf))
	})

	t.Run("modtime selects an object inside the window", func(t *testing.T) {
		outside := newObject("old/outside.txt", "contents", baseTime.Add(2*time.Second))
		inside := newObject("old/inside.txt", "contents", baseTime.Add(500*time.Millisecond))
		src := newObject("new/file.txt", "contents", baseTime)
		matcher := trackrenames.NewMatcher(ctx, trackrenames.StrategyModtime, hash.None, time.Second)

		assert.True(t, matcher.Add(outside))
		assert.True(t, matcher.Add(inside))
		assert.Same(t, inside, matcher.Match(src))
	})

	t.Run("modtime window is strict", func(t *testing.T) {
		dst := newObject("old/file.txt", "contents", baseTime.Add(time.Second))
		src := newObject("new/file.txt", "contents", baseTime)
		matcher := trackrenames.NewMatcher(ctx, trackrenames.StrategyModtime, hash.None, time.Second)

		assert.True(t, matcher.Add(dst))
		assert.Nil(t, matcher.Match(src))
	})

	t.Run("hash errors do not create matches", func(t *testing.T) {
		dst := &hashErrorObject{MemoryObject: newObject("old/file.txt", "contents", baseTime)}
		src := newObject("new/file.txt", "contents", baseTime)
		matcher := trackrenames.NewMatcher(ctx, trackrenames.StrategyHash, hash.MD5, 0)

		assert.False(t, matcher.Add(dst))
		assert.Nil(t, matcher.Match(src))
	})

	t.Run("guaranteed matches count duplicate identifiers", func(t *testing.T) {
		sources := []trackrenames.Candidate{
			{Remote: "new/one.txt", Size: 8, Hash: "same"},
			{Remote: "new/two.txt", Size: 8, Hash: "same"},
			{Remote: "new/three.txt", Size: 8, Hash: "same"},
		}
		destinations := []trackrenames.Candidate{
			{Remote: "old/one.txt", Size: 8, Hash: "same"},
			{Remote: "old/two.txt", Size: 8, Hash: "same"},
		}

		assert.Equal(t, 2, trackrenames.CountGuaranteedMatches(trackrenames.StrategyHash, 0, sources, destinations))
	})

	t.Run("guaranteed modtime matches require an order-independent group", func(t *testing.T) {
		sources := []trackrenames.Candidate{
			{Remote: "new/one.txt", Size: 8, ModTime: baseTime.Add(-300 * time.Millisecond)},
			{Remote: "new/two.txt", Size: 8, ModTime: baseTime.Add(300 * time.Millisecond)},
		}
		destinations := []trackrenames.Candidate{
			{Remote: "old/one.txt", Size: 8, ModTime: baseTime.Add(-400 * time.Millisecond)},
			{Remote: "old/two.txt", Size: 8, ModTime: baseTime.Add(400 * time.Millisecond)},
		}

		assert.Equal(t, 2, trackrenames.CountGuaranteedMatches(trackrenames.StrategyModtime, time.Second, sources, destinations))
	})

	t.Run("ambiguous modtime groups are not counted", func(t *testing.T) {
		sources := []trackrenames.Candidate{
			{Remote: "new/one.txt", Size: 8, ModTime: baseTime.Add(-900 * time.Millisecond)},
			{Remote: "new/two.txt", Size: 8, ModTime: baseTime.Add(900 * time.Millisecond)},
		}
		destinations := []trackrenames.Candidate{
			{Remote: "old/one.txt", Size: 8, ModTime: baseTime.Add(-900 * time.Millisecond)},
			{Remote: "old/two.txt", Size: 8, ModTime: baseTime.Add(900 * time.Millisecond)},
		}

		assert.Zero(t, trackrenames.CountGuaranteedMatches(trackrenames.StrategyModtime, time.Second, sources, destinations))
	})
}

type hashErrorObject struct {
	*object.MemoryObject
}

func (o *hashErrorObject) Hash(context.Context, hash.Type) (string, error) {
	return "", errors.New("hash failed")
}

var _ fs.Object = (*hashErrorObject)(nil)
