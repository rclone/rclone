package walk

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	_ "github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fstest/mockdir"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/fstest/mockobject"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var errDirNotFound, errorBoom error

func init() {
	errDirNotFound = fserrors.FsError(fs.ErrorDirNotFound)
	fserrors.Count(errDirNotFound)
	errorBoom = fserrors.FsError(errors.New("boom"))
	fserrors.Count(errorBoom)
}

type (
	listResult struct {
		entries fs.DirEntries
		err     error
	}

	listResults map[string]listResult

	errorMap map[string]error

	listDirs struct {
		mu          sync.Mutex
		t           *testing.T
		fs          fs.Fs
		includeAll  bool
		results     listResults
		walkResults listResults
		walkErrors  errorMap
		finalError  error
		checkMaps   bool
		maxLevel    int
	}
)

func newListDirs(t *testing.T, f fs.Fs, includeAll bool, results listResults, walkErrors errorMap, finalError error) *listDirs {
	return &listDirs{
		t:           t,
		fs:          f,
		includeAll:  includeAll,
		results:     results,
		walkErrors:  walkErrors,
		walkResults: listResults{},
		finalError:  finalError,
		checkMaps:   true,
		maxLevel:    -1,
	}
}

// NoCheckMaps marks the maps as to be ignored at the end
func (ls *listDirs) NoCheckMaps() *listDirs {
	ls.checkMaps = false
	return ls
}

// SetLevel(1) turns off recursion
func (ls *listDirs) SetLevel(maxLevel int) *listDirs {
	ls.maxLevel = maxLevel
	return ls
}

// ListDir returns the expected listing for the directory
func (ls *listDirs) ListDir(ctx context.Context, f fs.Fs, includeAll bool, dir string) (entries fs.DirEntries, err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	assert.Equal(ls.t, ls.fs, f)
	assert.Equal(ls.t, ls.includeAll, includeAll)

	// Fetch results for this path
	result, ok := ls.results[dir]
	if !ok {
		ls.t.Errorf("Unexpected list of %q", dir)
		return nil, errors.New("unexpected list")
	}
	delete(ls.results, dir)

	// Put expected results for call of WalkFn
	ls.walkResults[dir] = result

	return result.entries, result.err
}

// ListR returns the expected listing for the directory using ListR
func (ls *listDirs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	var errorReturn error
	for dirPath, result := range ls.results {
		// Put expected results for call of WalkFn
		// Note that we don't call the function at all if we got an error
		if result.err != nil {
			errorReturn = result.err
		}
		if errorReturn == nil {
			err = callback(result.entries)
			require.NoError(ls.t, err)
			ls.walkResults[dirPath] = result
		}
	}
	ls.results = listResults{}
	return errorReturn
}

// IsFinished checks everything expected was used up
func (ls *listDirs) IsFinished() {
	if ls.checkMaps {
		assert.Equal(ls.t, errorMap{}, ls.walkErrors)
		assert.Equal(ls.t, listResults{}, ls.results)
		assert.Equal(ls.t, listResults{}, ls.walkResults)
	}
}

// WalkFn is called by the walk to test the expectations
func (ls *listDirs) WalkFn(dir string, entries fs.DirEntries, err error) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	// ls.t.Logf("WalkFn(%q, %v, %q)", dir, entries, err)

	// Fetch expected entries and err
	result, ok := ls.walkResults[dir]
	if !ok {
		ls.t.Errorf("Unexpected walk of %q (result not found)", dir)
		return errors.New("result not found")
	}
	delete(ls.walkResults, dir)

	// Check arguments are as expected
	assert.Equal(ls.t, result.entries, entries)
	assert.Equal(ls.t, result.err, err)

	// Fetch return value
	returnErr, ok := ls.walkErrors[dir]
	if !ok {
		ls.t.Errorf("Unexpected walk of %q (error not found)", dir)
		return errors.New("error not found")
	}
	delete(ls.walkErrors, dir)

	return returnErr
}

// Walk does the walk and tests the expectations
func (ls *listDirs) Walk() {
	err := walk(context.Background(), nil, "", ls.includeAll, ls.maxLevel, ls.WalkFn, ls.ListDir)
	assert.Equal(ls.t, ls.finalError, err)
	ls.IsFinished()
}

// WalkR does the walkR and tests the expectations
func (ls *listDirs) WalkR() {
	err := walkR(context.Background(), nil, "", ls.includeAll, ls.maxLevel, ls.WalkFn, ls.ListR)
	assert.Equal(ls.t, ls.finalError, err)
	if ls.finalError == nil {
		ls.IsFinished()
	}
}

func testWalkEmpty(t *testing.T) *listDirs {
	return newListDirs(t, nil, false,
		listResults{
			"": {entries: fs.DirEntries{}, err: nil},
		},
		errorMap{
			"": nil,
		},
		nil,
	)
}
func TestWalkEmpty(t *testing.T)  { testWalkEmpty(t).Walk() }
func TestWalkREmpty(t *testing.T) { testWalkEmpty(t).WalkR() }

func testWalkEmptySkip(t *testing.T) *listDirs {
	return newListDirs(t, nil, true,
		listResults{
			"": {entries: fs.DirEntries{}, err: nil},
		},
		errorMap{
			"": ErrorSkipDir,
		},
		nil,
	)
}
func TestWalkEmptySkip(t *testing.T)  { testWalkEmptySkip(t).Walk() }
func TestWalkREmptySkip(t *testing.T) { testWalkEmptySkip(t).WalkR() }

func testWalkNotFound(t *testing.T) *listDirs {
	return newListDirs(t, nil, true,
		listResults{
			"": {err: errDirNotFound},
		},
		errorMap{
			"": errDirNotFound,
		},
		errDirNotFound,
	)
}
func TestWalkNotFound(t *testing.T)  { testWalkNotFound(t).Walk() }
func TestWalkRNotFound(t *testing.T) { testWalkNotFound(t).WalkR() }

func TestWalkNotFoundMaskError(t *testing.T) {
	// this doesn't work for WalkR
	newListDirs(t, nil, true,
		listResults{
			"": {err: errDirNotFound},
		},
		errorMap{
			"": nil,
		},
		nil,
	).Walk()
}

func TestWalkNotFoundSkipError(t *testing.T) {
	// this doesn't work for WalkR
	newListDirs(t, nil, true,
		listResults{
			"": {err: errDirNotFound},
		},
		errorMap{
			"": ErrorSkipDir,
		},
		nil,
	).Walk()
}

func testWalkLevels(t *testing.T, maxLevel int) *listDirs {
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	db := mockdir.New("a/b")
	oB := mockobject.Object("a/B")
	dc := mockdir.New("a/b/c")
	oC := mockobject.Object("a/b/C")
	dd := mockdir.New("a/b/c/d")
	oD := mockobject.Object("a/b/c/D")
	return newListDirs(t, nil, false,
		listResults{
			"":        {entries: fs.DirEntries{oA, da}, err: nil},
			"a":       {entries: fs.DirEntries{oB, db}, err: nil},
			"a/b":     {entries: fs.DirEntries{oC, dc}, err: nil},
			"a/b/c":   {entries: fs.DirEntries{oD, dd}, err: nil},
			"a/b/c/d": {entries: fs.DirEntries{}, err: nil},
		},
		errorMap{
			"":        nil,
			"a":       nil,
			"a/b":     nil,
			"a/b/c":   nil,
			"a/b/c/d": nil,
		},
		nil,
	).SetLevel(maxLevel)
}
func TestWalkLevels(t *testing.T)               { testWalkLevels(t, -1).Walk() }
func TestWalkRLevels(t *testing.T)              { testWalkLevels(t, -1).WalkR() }
func TestWalkLevelsNoRecursive10(t *testing.T)  { testWalkLevels(t, 10).Walk() }
func TestWalkRLevelsNoRecursive10(t *testing.T) { testWalkLevels(t, 10).WalkR() }

func TestWalkNDirTree(t *testing.T) {
	ls := testWalkLevels(t, -1)
	entries, err := walkNDirTree(context.Background(), nil, "", ls.includeAll, ls.maxLevel, ls.ListDir)
	require.NoError(t, err)
	assert.Equal(t, `/
  A
  a/
a/
  B
  b/
a/b/
  C
  c/
a/b/c/
  D
  d/
a/b/c/d/
`, entries.String())
}

func testWalkLevelsNoRecursive(t *testing.T) *listDirs {
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	return newListDirs(t, nil, false,
		listResults{
			"": {entries: fs.DirEntries{oA, da}, err: nil},
		},
		errorMap{
			"": nil,
		},
		nil,
	).SetLevel(1)
}
func TestWalkLevelsNoRecursive(t *testing.T)  { testWalkLevelsNoRecursive(t).Walk() }
func TestWalkRLevelsNoRecursive(t *testing.T) { testWalkLevelsNoRecursive(t).WalkR() }

func testWalkLevels2(t *testing.T) *listDirs {
	da := mockdir.New("a")
	oA := mockobject.Object("A")
	db := mockdir.New("a/b")
	oB := mockobject.Object("a/B")
	return newListDirs(t, nil, false,
		listResults{
			"":  {entries: fs.DirEntries{oA, da}, err: nil},
			"a": {entries: fs.DirEntries{oB, db}, err: nil},
		},
		errorMap{
			"":  nil,
			"a": nil,
		},
		nil,
	).SetLevel(2)
}
func TestWalkLevels2(t *testing.T)  { testWalkLevels2(t).Walk() }
func TestWalkRLevels2(t *testing.T) { testWalkLevels2(t).WalkR() }

func testWalkSkip(t *testing.T) *listDirs {
	da := mockdir.New("a")
	db := mockdir.New("a/b")
	dc := mockdir.New("a/b/c")
	return newListDirs(t, nil, false,
		listResults{
			"":    {entries: fs.DirEntries{da}, err: nil},
			"a":   {entries: fs.DirEntries{db}, err: nil},
			"a/b": {entries: fs.DirEntries{dc}, err: nil},
		},
		errorMap{
			"":    nil,
			"a":   nil,
			"a/b": ErrorSkipDir,
		},
		nil,
	)
}
func TestWalkSkip(t *testing.T)  { testWalkSkip(t).Walk() }
func TestWalkRSkip(t *testing.T) { testWalkSkip(t).WalkR() }

func walkErrors(t *testing.T, expectedErr error) *listDirs {
	lr := listResults{}
	em := errorMap{}
	de := make(fs.DirEntries, 10)
	for i := range de {
		path := string('0' + rune(i))
		de[i] = mockdir.New(path)
		lr[path] = listResult{entries: nil, err: fs.ErrorDirNotFound}
		em[path] = fs.ErrorDirNotFound
	}
	lr[""] = listResult{entries: de, err: nil}
	em[""] = nil
	return newListDirs(t, nil, true,
		lr,
		em,
		expectedErr,
	).NoCheckMaps()
}

func testWalkErrors(t *testing.T) *listDirs {
	return walkErrors(t, errDirNotFound)
}

func testWalkRErrors(t *testing.T) *listDirs {
	return walkErrors(t, fs.ErrorDirNotFound)
}

func TestWalkErrors(t *testing.T)  { testWalkErrors(t).Walk() }
func TestWalkRErrors(t *testing.T) { testWalkRErrors(t).WalkR() }

func makeTree(level int, terminalErrors bool) (listResults, errorMap) {
	lr := listResults{}
	em := errorMap{}
	var fill func(path string, level int)
	fill = func(path string, level int) {
		de := fs.DirEntries{}
		if level > 0 {
			for _, a := range "0123456789" {
				subPath := string(a)
				if path != "" {
					subPath = path + "/" + subPath
				}
				de = append(de, mockdir.New(subPath))
				fill(subPath, level-1)
			}
		}
		lr[path] = listResult{entries: de, err: nil}
		em[path] = nil
		if level == 0 && terminalErrors {
			em[path] = errorBoom
		}
	}
	fill("", level)
	return lr, em
}

func testWalkMulti(t *testing.T) *listDirs {
	lr, em := makeTree(3, false)
	return newListDirs(t, nil, true,
		lr,
		em,
		nil,
	)
}
func TestWalkMulti(t *testing.T)  { testWalkMulti(t).Walk() }
func TestWalkRMulti(t *testing.T) { testWalkMulti(t).WalkR() }

func testWalkMultiErrors(t *testing.T) *listDirs {
	lr, em := makeTree(3, true)
	return newListDirs(t, nil, true,
		lr,
		em,
		errorBoom,
	).NoCheckMaps()
}
func TestWalkMultiErrors(t *testing.T)  { testWalkMultiErrors(t).Walk() }
func TestWalkRMultiErrors(t *testing.T) { testWalkMultiErrors(t).Walk() }

// a very simple listRcallback function
func makeListRCallback(entries fs.DirEntries, err error) fs.ListRFn {
	return func(ctx context.Context, dir string, callback fs.ListRCallback) error {
		if err == nil {
			err = callback(entries)
		}
		return err
	}
}

func TestWalkRDirTree(t *testing.T) {
	for _, test := range []struct {
		entries fs.DirEntries
		want    string
		err     error
		root    string
		level   int
	}{
		{fs.DirEntries{}, "/\n", nil, "", -1},
		{fs.DirEntries{mockobject.Object("a")}, `/
  a
`, nil, "", -1},
		{fs.DirEntries{mockobject.Object("a/b")}, `/
  a/
a/
  b
`, nil, "", -1},
		{fs.DirEntries{mockobject.Object("a/b/c/d")}, `/
  a/
a/
  b/
a/b/
  c/
a/b/c/
  d
`, nil, "", -1},
		{fs.DirEntries{mockobject.Object("a")}, "", errorBoom, "", -1},
		{fs.DirEntries{
			mockobject.Object("0/1/2/3"),
			mockobject.Object("4/5/6/7"),
			mockobject.Object("8/9/a/b"),
			mockobject.Object("c/d/e/f"),
			mockobject.Object("g/h/i/j"),
			mockobject.Object("k/l/m/n"),
			mockobject.Object("o/p/q/r"),
			mockobject.Object("s/t/u/v"),
			mockobject.Object("w/x/y/z"),
		}, `/
  0/
  4/
  8/
  c/
  g/
  k/
  o/
  s/
  w/
0/
  1/
0/1/
  2/
0/1/2/
  3
4/
  5/
4/5/
  6/
4/5/6/
  7
8/
  9/
8/9/
  a/
8/9/a/
  b
c/
  d/
c/d/
  e/
c/d/e/
  f
g/
  h/
g/h/
  i/
g/h/i/
  j
k/
  l/
k/l/
  m/
k/l/m/
  n
o/
  p/
o/p/
  q/
o/p/q/
  r
s/
  t/
s/t/
  u/
s/t/u/
  v
w/
  x/
w/x/
  y/
w/x/y/
  z
`, nil, "", -1},
		{fs.DirEntries{
			mockobject.Object("a/b/c/d/e/f1"),
			mockobject.Object("a/b/c/d/e/f2"),
			mockobject.Object("a/b/c/d/e/f3"),
		}, `a/b/c/
  d/
a/b/c/d/
  e/
a/b/c/d/e/
  f1
  f2
  f3
`, nil, "a/b/c", -1},
		{fs.DirEntries{
			mockobject.Object("A"),
			mockobject.Object("a/B"),
			mockobject.Object("a/b/C"),
			mockobject.Object("a/b/c/D"),
			mockobject.Object("a/b/c/d/E"),
		}, `/
  A
  a/
a/
  B
  b/
`, nil, "", 2},
		{fs.DirEntries{
			mockobject.Object("a/b/c"),
			mockobject.Object("a/b/c/d/e"),
		}, `/
  a/
a/
  b/
`, nil, "", 2},
	} {
		r, err := walkRDirTree(context.Background(), nil, test.root, true, test.level, makeListRCallback(test.entries, test.err))
		assert.Equal(t, test.err, err, fmt.Sprintf("%+v", test))
		assert.Equal(t, test.want, r.String(), fmt.Sprintf("%+v", test))
	}
}

func TestWalkRDirTreeExclude(t *testing.T) {
	for _, test := range []struct {
		entries     fs.DirEntries
		want        string
		err         error
		root        string
		level       int
		excludeFile string
		includeAll  bool
	}{
		{fs.DirEntries{mockobject.Object("a"), mockobject.Object("ignore")}, "", nil, "", -1, "ignore", false},
		{fs.DirEntries{mockobject.Object("a")}, `/
  a
`, nil, "", -1, "ignore", false},
		{fs.DirEntries{
			mockobject.Object("a"),
			mockobject.Object("b/b"),
			mockobject.Object("b/.ignore"),
		}, `/
  a
`, nil, "", -1, ".ignore", false},
		{fs.DirEntries{
			mockobject.Object("a"),
			mockobject.Object("b/.ignore"),
			mockobject.Object("b/b"),
		}, `/
  a
  b/
b/
  .ignore
  b
`, nil, "", -1, ".ignore", true},
		{fs.DirEntries{
			mockobject.Object("a"),
			mockobject.Object("b/b"),
			mockobject.Object("b/c/d/e"),
			mockobject.Object("b/c/ign"),
			mockobject.Object("b/c/x"),
		}, `/
  a
  b/
b/
  b
`, nil, "", -1, "ign", false},
		{fs.DirEntries{
			mockobject.Object("a"),
			mockobject.Object("b/b"),
			mockobject.Object("b/c/d/e"),
			mockobject.Object("b/c/ign"),
			mockobject.Object("b/c/x"),
		}, `/
  a
  b/
b/
  b
  c/
b/c/
  d/
  ign
  x
b/c/d/
  e
`, nil, "", -1, "ign", true},
	} {
		filter.Active.Opt.ExcludeFile = test.excludeFile
		r, err := walkRDirTree(context.Background(), nil, test.root, test.includeAll, test.level, makeListRCallback(test.entries, test.err))
		assert.Equal(t, test.err, err, fmt.Sprintf("%+v", test))
		assert.Equal(t, test.want, r.String(), fmt.Sprintf("%+v", test))
	}
	// Set to default value, to avoid side effects
	filter.Active.Opt.ExcludeFile = ""
}

func TestListType(t *testing.T) {
	assert.Equal(t, true, ListObjects.Objects())
	assert.Equal(t, false, ListObjects.Dirs())
	assert.Equal(t, false, ListDirs.Objects())
	assert.Equal(t, true, ListDirs.Dirs())
	assert.Equal(t, true, ListAll.Objects())
	assert.Equal(t, true, ListAll.Dirs())

	var (
		a           = mockobject.Object("a")
		b           = mockobject.Object("b")
		dir         = mockdir.New("dir")
		adir        = mockobject.Object("dir/a")
		dir2        = mockdir.New("dir2")
		origEntries = fs.DirEntries{
			a, b, dir, adir, dir2,
		}
		dirEntries = fs.DirEntries{
			dir, dir2,
		}
		objEntries = fs.DirEntries{
			a, b, adir,
		}
	)
	copyOrigEntries := func() (out fs.DirEntries) {
		out = make(fs.DirEntries, len(origEntries))
		copy(out, origEntries)
		return out
	}

	got := copyOrigEntries()
	ListAll.Filter(&got)
	assert.Equal(t, origEntries, got)

	got = copyOrigEntries()
	ListObjects.Filter(&got)
	assert.Equal(t, objEntries, got)

	got = copyOrigEntries()
	ListDirs.Filter(&got)
	assert.Equal(t, dirEntries, got)
}

func TestListR(t *testing.T) {
	objects := fs.DirEntries{
		mockobject.Object("a"),
		mockobject.Object("b"),
		mockdir.New("dir"),
		mockobject.Object("dir/a"),
		mockobject.Object("dir/b"),
		mockobject.Object("dir/c"),
	}
	f := mockfs.NewFs("mock", "/")
	var got []string
	clearCallback := func() {
		got = nil
	}
	callback := func(entries fs.DirEntries) error {
		for _, entry := range entries {
			got = append(got, entry.Remote())
		}
		return nil
	}
	doListR := func(ctx context.Context, dir string, callback fs.ListRCallback) error {
		var os fs.DirEntries
		for _, o := range objects {
			if dir == "" || strings.HasPrefix(o.Remote(), dir+"/") {
				os = append(os, o)
			}
		}
		return callback(os)
	}

	// Setup filter
	oldFilter := filter.Active
	defer func() {
		filter.Active = oldFilter
	}()

	var err error
	filter.Active, err = filter.NewFilter(nil)
	require.NoError(t, err)
	require.NoError(t, filter.Active.AddRule("+ b"))
	require.NoError(t, filter.Active.AddRule("- *"))

	// Base case
	clearCallback()
	err = listR(context.Background(), f, "", true, ListAll, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "dir", "dir/a", "dir/b", "dir/c"}, got)

	// Base case - with Objects
	clearCallback()
	err = listR(context.Background(), f, "", true, ListObjects, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "dir/a", "dir/b", "dir/c"}, got)

	// Base case - with Dirs
	clearCallback()
	err = listR(context.Background(), f, "", true, ListDirs, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"dir"}, got)

	// With filter
	clearCallback()
	err = listR(context.Background(), f, "", false, ListAll, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "dir", "dir/b"}, got)

	// With filter - with Objects
	clearCallback()
	err = listR(context.Background(), f, "", false, ListObjects, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "dir/b"}, got)

	// With filter - with Dir
	clearCallback()
	err = listR(context.Background(), f, "", false, ListDirs, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"dir"}, got)

	// With filter and subdir
	clearCallback()
	err = listR(context.Background(), f, "dir", false, ListAll, callback, doListR, false)
	require.NoError(t, err)
	require.Equal(t, []string{"dir/b"}, got)

	// Now bucket based
	objects = fs.DirEntries{
		mockobject.Object("a"),
		mockobject.Object("b"),
		mockobject.Object("dir/a"),
		mockobject.Object("dir/b"),
		mockobject.Object("dir/subdir/c"),
		mockdir.New("dir/subdir"),
	}

	// Base case
	clearCallback()
	err = listR(context.Background(), f, "", true, ListAll, callback, doListR, true)
	require.NoError(t, err)
	require.Equal(t, []string{"a", "b", "dir/a", "dir/b", "dir/subdir/c", "dir/subdir", "dir"}, got)

	// With filter
	clearCallback()
	err = listR(context.Background(), f, "", false, ListAll, callback, doListR, true)
	require.NoError(t, err)
	require.Equal(t, []string{"b", "dir/b", "dir/subdir", "dir"}, got)

	// With filter and subdir
	clearCallback()
	err = listR(context.Background(), f, "dir", false, ListAll, callback, doListR, true)
	require.NoError(t, err)
	require.Equal(t, []string{"dir/b", "dir/subdir"}, got)

	// With filter and subdir - with Objects
	clearCallback()
	err = listR(context.Background(), f, "dir", false, ListObjects, callback, doListR, true)
	require.NoError(t, err)
	require.Equal(t, []string{"dir/b"}, got)

	// With filter and subdir - with Dirs
	clearCallback()
	err = listR(context.Background(), f, "dir", false, ListDirs, callback, doListR, true)
	require.NoError(t, err)
	require.Equal(t, []string{"dir/subdir"}, got)
}

func TestDirMapAdd(t *testing.T) {
	type add struct {
		dir  string
		sent bool
	}
	for i, test := range []struct {
		root string
		in   []add
		want map[string]bool
	}{
		{
			root: "",
			in: []add{
				{"", true},
			},
			want: map[string]bool{},
		},
		{
			root: "",
			in: []add{
				{"a/b/c", true},
			},
			want: map[string]bool{
				"a/b/c": true,
				"a/b":   false,
				"a":     false,
			},
		},
		{
			root: "",
			in: []add{
				{"a/b/c", true},
				{"a/b", true},
			},
			want: map[string]bool{
				"a/b/c": true,
				"a/b":   true,
				"a":     false,
			},
		},
		{
			root: "",
			in: []add{
				{"a/b", true},
				{"a/b/c", false},
			},
			want: map[string]bool{
				"a/b/c": false,
				"a/b":   true,
				"a":     false,
			},
		},
		{
			root: "root",
			in: []add{
				{"root/a/b", true},
				{"root/a/b/c", false},
			},
			want: map[string]bool{
				"root/a/b/c": false,
				"root/a/b":   true,
				"root/a":     false,
			},
		},
	} {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			dm := newDirMap(test.root)
			for _, item := range test.in {
				dm.add(item.dir, item.sent)
			}
			assert.Equal(t, test.want, dm.m)
		})
	}
}

func TestDirMapAddEntries(t *testing.T) {
	dm := newDirMap("")
	entries := fs.DirEntries{
		mockobject.Object("dir/a"),
		mockobject.Object("dir/b"),
		mockdir.New("dir"),
		mockobject.Object("dir2/a"),
		mockobject.Object("dir2/b"),
	}
	require.NoError(t, dm.addEntries(entries))
	assert.Equal(t, map[string]bool{"dir": true, "dir2": false}, dm.m)
}

func TestDirMapSendEntries(t *testing.T) {
	var got []string
	clearCallback := func() {
		got = nil
	}
	callback := func(entries fs.DirEntries) error {
		for _, entry := range entries {
			got = append(got, entry.Remote())
		}
		return nil
	}

	// general test
	dm := newDirMap("")
	entries := fs.DirEntries{
		mockobject.Object("dir/a"),
		mockobject.Object("dir/b"),
		mockdir.New("dir"),
		mockobject.Object("dir2/a"),
		mockobject.Object("dir2/b"),
		mockobject.Object("dir1/a"),
		mockobject.Object("dir3/b"),
	}
	require.NoError(t, dm.addEntries(entries))
	clearCallback()
	err := dm.sendEntries(callback)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"dir1",
		"dir2",
		"dir3",
	}, got)

	// return error from callback
	callback2 := func(entries fs.DirEntries) error {
		return io.EOF
	}
	err = dm.sendEntries(callback2)
	require.Equal(t, io.EOF, err)

	// empty
	dm = newDirMap("")
	clearCallback()
	err = dm.sendEntries(callback)
	require.NoError(t, err)
	assert.Equal(t, []string(nil), got)
}
