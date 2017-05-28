package fs

import (
	"io"
	"sort"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListerNew(t *testing.T) {
	o := NewLister()
	assert.Equal(t, Config.Checkers, o.buffer)
	assert.Equal(t, false, o.abort)
	assert.Equal(t, MaxLevel, o.level)
}

var errNotImpl = errors.New("not implemented")

type mockObject string

func (o mockObject) String() string                                    { return string(o) }
func (o mockObject) Fs() Info                                          { return nil }
func (o mockObject) Remote() string                                    { return string(o) }
func (o mockObject) Hash(HashType) (string, error)                     { return "", errNotImpl }
func (o mockObject) ModTime() (t time.Time)                            { return t }
func (o mockObject) Size() int64                                       { return 0 }
func (o mockObject) Storable() bool                                    { return true }
func (o mockObject) SetModTime(time.Time) error                        { return errNotImpl }
func (o mockObject) Open(options ...OpenOption) (io.ReadCloser, error) { return nil, errNotImpl }
func (o mockObject) Update(in io.Reader, src ObjectInfo, options ...OpenOption) error {
	return errNotImpl
}
func (o mockObject) Remove() error { return errNotImpl }

type mockFs struct {
	listFn func(o ListOpts, dir string)
}

func (f *mockFs) List(o ListOpts, dir string) {
	defer o.Finished()
	f.listFn(o, dir)
}

func (f *mockFs) NewObject(remote string) (Object, error) {
	return mockObject(remote), nil
}

func TestListerStart(t *testing.T) {
	f := &mockFs{}
	ranList := false
	f.listFn = func(o ListOpts, dir string) {
		ranList = true
	}
	o := NewLister().Start(f, "")
	objs, dirs, err := o.GetAll()
	require.Nil(t, err)
	assert.Len(t, objs, 0)
	assert.Len(t, dirs, 0)
	assert.Equal(t, true, ranList)
}

func TestListerStartWithFiles(t *testing.T) {
	f := &mockFs{}
	ranList := false
	f.listFn = func(o ListOpts, dir string) {
		ranList = true
	}
	filter, err := NewFilter()
	require.NoError(t, err)
	wantNames := []string{"potato", "sausage", "rutabaga", "carrot", "lettuce"}
	sort.Strings(wantNames)
	for _, name := range wantNames {
		err = filter.AddFile(name)
		require.NoError(t, err)
	}
	o := NewLister().SetFilter(filter).Start(f, "")
	objs, dirs, err := o.GetAll()
	require.Nil(t, err)
	assert.Len(t, dirs, 0)
	assert.Equal(t, false, ranList)
	var gotNames []string
	for _, obj := range objs {
		gotNames = append(gotNames, obj.Remote())
	}
	sort.Strings(gotNames)
	assert.Equal(t, wantNames, gotNames)
}

func TestListerSetLevel(t *testing.T) {
	o := NewLister()
	o.SetLevel(1)
	assert.Equal(t, 1, o.level)
	o.SetLevel(0)
	assert.Equal(t, 0, o.level)
	o.SetLevel(-1)
	assert.Equal(t, MaxLevel, o.level)
}

func TestListerSetFilter(t *testing.T) {
	filter := &Filter{}
	o := NewLister().SetFilter(filter)
	assert.Equal(t, filter, o.filter)
}

func TestListerLevel(t *testing.T) {
	o := NewLister()
	assert.Equal(t, MaxLevel, o.Level())
	o.SetLevel(123)
	assert.Equal(t, 123, o.Level())
}

func TestListerSetBuffer(t *testing.T) {
	o := NewLister()
	o.SetBuffer(2)
	assert.Equal(t, 2, o.buffer)
	o.SetBuffer(1)
	assert.Equal(t, 1, o.buffer)
	o.SetBuffer(0)
	assert.Equal(t, 1, o.buffer)
	o.SetBuffer(-1)
	assert.Equal(t, 1, o.buffer)
}

func TestListerBuffer(t *testing.T) {
	o := NewLister()
	assert.Equal(t, Config.Checkers, o.Buffer())
	o.SetBuffer(123)
	assert.Equal(t, 123, o.Buffer())
}

func TestListerAdd(t *testing.T) {
	f := &mockFs{}
	objs := []Object{
		mockObject("1"),
		mockObject("2"),
	}
	f.listFn = func(o ListOpts, dir string) {
		for _, obj := range objs {
			assert.Equal(t, false, o.Add(obj))
		}
	}
	o := NewLister().Start(f, "")
	gotObjs, gotDirs, err := o.GetAll()
	require.Nil(t, err)
	assert.Equal(t, objs, gotObjs)
	assert.Len(t, gotDirs, 0)
}

func TestListerAddDir(t *testing.T) {
	f := &mockFs{}
	dirs := []*Dir{
		&Dir{Name: "1"},
		&Dir{Name: "2"},
	}
	f.listFn = func(o ListOpts, dir string) {
		for _, dir := range dirs {
			assert.Equal(t, false, o.AddDir(dir))
		}
	}
	o := NewLister().Start(f, "")
	gotObjs, gotDirs, err := o.GetAll()
	require.Nil(t, err)
	assert.Len(t, gotObjs, 0)
	assert.Equal(t, dirs, gotDirs)
}

func TestListerIncludeDirectory(t *testing.T) {
	o := NewLister()
	assert.Equal(t, true, o.IncludeDirectory("whatever"))
	filter, err := NewFilter()
	require.Nil(t, err)
	require.NotNil(t, filter)
	require.Nil(t, filter.AddRule("!"))
	require.Nil(t, filter.AddRule("+ potato/*"))
	require.Nil(t, filter.AddRule("- *"))
	o.SetFilter(filter)
	assert.Equal(t, false, o.IncludeDirectory("floop"))
	assert.Equal(t, true, o.IncludeDirectory("potato"))
	assert.Equal(t, false, o.IncludeDirectory("potato/sausage"))
}

func TestListerSetError(t *testing.T) {
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		assert.Equal(t, false, o.Add(mockObject("1")))
		o.SetError(errNotImpl)
		assert.Equal(t, true, o.Add(mockObject("2")))
		o.SetError(errors.New("not signalled"))
		assert.Equal(t, true, o.AddDir(&Dir{Name: "2"}))
	}
	o := NewLister().Start(f, "")
	gotObjs, gotDirs, err := o.GetAll()
	assert.Equal(t, err, errNotImpl)
	assert.Nil(t, gotObjs)
	assert.Nil(t, gotDirs)
}

func TestListerIsFinished(t *testing.T) {
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		assert.Equal(t, false, o.IsFinished())
		o.Finished()
		assert.Equal(t, true, o.IsFinished())
	}
	o := NewLister().Start(f, "")
	gotObjs, gotDirs, err := o.GetAll()
	assert.Nil(t, err)
	assert.Len(t, gotObjs, 0)
	assert.Len(t, gotDirs, 0)
}

func testListerGet(t *testing.T) *Lister {
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		assert.Equal(t, false, o.Add(mockObject("1")))
		assert.Equal(t, false, o.AddDir(&Dir{Name: "2"}))
	}
	return NewLister().Start(f, "")
}

func TestListerGet(t *testing.T) {
	o := testListerGet(t)
	obj, dir, err := o.Get()
	assert.Nil(t, err)
	assert.Equal(t, obj.Remote(), "1")
	assert.Nil(t, dir)
	obj, dir, err = o.Get()
	assert.Nil(t, err)
	assert.Nil(t, obj)
	assert.Equal(t, dir.Name, "2")
	obj, dir, err = o.Get()
	assert.Nil(t, err)
	assert.Nil(t, obj)
	assert.Nil(t, dir)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetObject(t *testing.T) {
	o := testListerGet(t)
	obj, err := o.GetObject()
	assert.Nil(t, err)
	assert.Equal(t, obj.Remote(), "1")
	obj, err = o.GetObject()
	assert.Nil(t, err)
	assert.Nil(t, obj)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetDir(t *testing.T) {
	o := testListerGet(t)
	dir, err := o.GetDir()
	assert.Nil(t, err)
	assert.Equal(t, dir.Name, "2")
	dir, err = o.GetDir()
	assert.Nil(t, err)
	assert.Nil(t, dir)
	assert.Equal(t, true, o.IsFinished())
}

func testListerGetError(t *testing.T) *Lister {
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		o.SetError(errNotImpl)
	}
	return NewLister().Start(f, "")
}

func TestListerGetError(t *testing.T) {
	o := testListerGetError(t)
	obj, dir, err := o.Get()
	assert.Equal(t, err, errNotImpl)
	assert.Nil(t, obj)
	assert.Nil(t, dir)
	obj, dir, err = o.Get()
	assert.Nil(t, err)
	assert.Nil(t, obj)
	assert.Nil(t, dir)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetObjectError(t *testing.T) {
	o := testListerGetError(t)
	obj, err := o.GetObject()
	assert.Equal(t, err, errNotImpl)
	assert.Nil(t, obj)
	obj, err = o.GetObject()
	assert.Nil(t, err)
	assert.Nil(t, obj)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetDirError(t *testing.T) {
	o := testListerGetError(t)
	dir, err := o.GetDir()
	assert.Equal(t, err, errNotImpl)
	assert.Nil(t, dir)
	dir, err = o.GetDir()
	assert.Nil(t, err)
	assert.Nil(t, dir)
	assert.Equal(t, true, o.IsFinished())
}

func testListerGetAll(t *testing.T) (*Lister, []Object, []*Dir) {
	objs := []Object{
		mockObject("1f"),
		mockObject("2f"),
		mockObject("3f"),
	}
	dirs := []*Dir{
		&Dir{Name: "1d"},
		&Dir{Name: "2d"},
	}
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		assert.Equal(t, false, o.Add(objs[0]))
		assert.Equal(t, false, o.Add(objs[1]))
		assert.Equal(t, false, o.AddDir(dirs[0]))
		assert.Equal(t, false, o.Add(objs[2]))
		assert.Equal(t, false, o.AddDir(dirs[1]))
	}
	return NewLister().Start(f, ""), objs, dirs
}

func TestListerGetAll(t *testing.T) {
	o, objs, dirs := testListerGetAll(t)
	gotObjs, gotDirs, err := o.GetAll()
	assert.Nil(t, err)
	assert.Equal(t, objs, gotObjs)
	assert.Equal(t, dirs, gotDirs)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetObjects(t *testing.T) {
	o, objs, _ := testListerGetAll(t)
	gotObjs, err := o.GetObjects()
	assert.Nil(t, err)
	assert.Equal(t, objs, gotObjs)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetDirs(t *testing.T) {
	o, _, dirs := testListerGetAll(t)
	gotDirs, err := o.GetDirs()
	assert.Nil(t, err)
	assert.Equal(t, dirs, gotDirs)
	assert.Equal(t, true, o.IsFinished())
}

func testListerGetAllError(t *testing.T) *Lister {
	f := &mockFs{}
	f.listFn = func(o ListOpts, dir string) {
		o.SetError(errNotImpl)
	}
	return NewLister().Start(f, "")
}

func TestListerGetAllError(t *testing.T) {
	o := testListerGetAllError(t)
	gotObjs, gotDirs, err := o.GetAll()
	assert.Equal(t, errNotImpl, err)
	assert.Len(t, gotObjs, 0)
	assert.Len(t, gotDirs, 0)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetObjectsError(t *testing.T) {
	o := testListerGetAllError(t)
	gotObjs, err := o.GetObjects()
	assert.Equal(t, errNotImpl, err)
	assert.Len(t, gotObjs, 0)
	assert.Equal(t, true, o.IsFinished())
}

func TestListerGetDirsError(t *testing.T) {
	o := testListerGetAllError(t)
	gotDirs, err := o.GetDirs()
	assert.Equal(t, errNotImpl, err)
	assert.Len(t, gotDirs, 0)
	assert.Equal(t, true, o.IsFinished())
}
