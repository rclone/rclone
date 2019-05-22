package fs

import (
	"fmt"
	"io"
	"sort"
	"testing"
	"time"

	"github.com/ncw/rclone/fs/hash"
	"github.com/stretchr/testify/assert"
)

type FakeObject string

func (o FakeObject) ModTime() time.Time {
	return time.Now()
}

func (o FakeObject) String() string {
	return fmt.Sprintf("FakeObject(%s)", string(o))
}

func (o FakeObject) Remote() string {
	return string(o)
}

func (o FakeObject) Size() int64 {
	return 0
}

func (o FakeObject) SetModTime(t time.Time) error {
	return nil
}

func (o FakeObject) Open(options ...OpenOption) (io.ReadCloser, error) {
	return nil, nil
}

func (o FakeObject) Update(in io.Reader, src ObjectInfo, options ...OpenOption) error {
	return nil
}

func (o FakeObject) Remove() error {
	return nil
}

func (o FakeObject) Fs() Info {
	return nil
}

func (o FakeObject) Hash(typ hash.Type) (string, error) {
	return string(o), nil
}

func (o FakeObject) Storable() bool {
	return false
}

type FakeDir string

func (d FakeDir) Items() int64 {
	return 0
}

func (d FakeDir) ID() string {
	return string(d)
}

func (d FakeDir) Remote() string {
	return string(d)
}

func (d FakeDir) String() string {
	return fmt.Sprintf("FakeDir(%s)", string(d))
}

func (d FakeDir) ModTime() time.Time {
	return time.Now()
}

func (d FakeDir) Size() int64 {
	return 0
}

var _ Object = FakeObject("")
var _ Directory = FakeDir("")

func TestDirEntriesSort(t *testing.T) {
	a := FakeObject("a")
	aDir := FakeDir("a")
	b := FakeObject("b")
	bDir := FakeDir("b")
	dirEntries := DirEntries{bDir, b, aDir, a}
	sort.Sort(dirEntries)
	assert.Equal(t, DirEntries{a, aDir, b, bDir}, dirEntries)
}
