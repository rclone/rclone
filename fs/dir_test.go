package fs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDirModTimeValid(t *testing.T) {
	t.Run("unknown modtime", func(t *testing.T) {
		d := NewDir("remote", time.Time{})
		assert.False(t, d.ModTimeValid())
	})
	t.Run("known modtime", func(t *testing.T) {
		when := time.Now()
		d := NewDir("remote", when)
		assert.True(t, d.ModTimeValid())
		assert.Equal(t, when, d.ModTime(context.Background()))
	})
}

// bareDirectory is a minimal fs.Directory which does not implement
// ModTimeValid, to test NewDirCopy's fallback for Directory
// implementations other than *Dir
type bareDirectory struct {
	remote  string
	modTime time.Time
}

func (b *bareDirectory) Fs() Info                          { return Unknown }
func (b *bareDirectory) String() string                    { return b.remote }
func (b *bareDirectory) Remote() string                    { return b.remote }
func (b *bareDirectory) ModTime(context.Context) time.Time { return b.modTime }
func (b *bareDirectory) Size() int64                       { return -1 }
func (b *bareDirectory) Items() int64                      { return -1 }
func (b *bareDirectory) ID() string                        { return "" }

var _ Directory = (*bareDirectory)(nil)

func TestNewDirCopyModTimeValid(t *testing.T) {
	ctx := context.Background()
	t.Run("copy of Dir with unknown modtime stays invalid", func(t *testing.T) {
		src := NewDir("remote", time.Time{})
		dst := NewDirCopy(ctx, src)
		assert.False(t, dst.ModTimeValid())
	})
	t.Run("copy of Dir with known modtime stays valid", func(t *testing.T) {
		src := NewDir("remote", time.Now())
		dst := NewDirCopy(ctx, src)
		assert.True(t, dst.ModTimeValid())
	})
	t.Run("copy of a Directory without ModTimeValid defaults to valid", func(t *testing.T) {
		src := &bareDirectory{remote: "remote", modTime: time.Time{}}
		dst := NewDirCopy(ctx, src)
		assert.True(t, dst.ModTimeValid())
	})
}
