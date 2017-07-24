package fs

import (
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type (
	listResult struct {
		entries DirEntries
		err     error
	}

	listResults map[string]listResult

	errorMap map[string]error

	listDirs struct {
		mu          sync.Mutex
		t           *testing.T
		fs          Fs
		includeAll  bool
		results     listResults
		walkResults listResults
		walkErrors  errorMap
		finalError  error
		checkMaps   bool
		maxLevel    int
	}
)

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

type unknownDirEntry string

func (o unknownDirEntry) String() string         { return string(o) }
func (o unknownDirEntry) Remote() string         { return string(o) }
func (o unknownDirEntry) ModTime() (t time.Time) { return t }
func (o unknownDirEntry) Size() int64            { return 0 }

func newListDirs(t *testing.T, f Fs, includeAll bool, results listResults, walkErrors errorMap, finalError error) *listDirs {
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
func (ls *listDirs) ListDir(f Fs, includeAll bool, dir string) (entries DirEntries, err error) {
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
func (ls *listDirs) ListR(dir string, callback ListRCallback) (err error) {
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
func (ls *listDirs) WalkFn(dir string, entries DirEntries, err error) error {
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
	err := walk(nil, "", ls.includeAll, ls.maxLevel, ls.WalkFn, ls.ListDir)
	assert.Equal(ls.t, ls.finalError, err)
	ls.IsFinished()
}

// WalkR does the walkR and tests the expectations
func (ls *listDirs) WalkR() {
	err := walkR(nil, "", ls.includeAll, ls.maxLevel, ls.WalkFn, ls.ListR)
	assert.Equal(ls.t, ls.finalError, err)
	if ls.finalError == nil {
		ls.IsFinished()
	}
}

func newDir(name string) Directory {
	return NewDir(name, time.Time{})
}

func testWalkEmpty(t *testing.T) *listDirs {
	return newListDirs(t, nil, false,
		listResults{
			"": {entries: DirEntries{}, err: nil},
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
			"": {entries: DirEntries{}, err: nil},
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
			"": {err: ErrorDirNotFound},
		},
		errorMap{
			"": ErrorDirNotFound,
		},
		ErrorDirNotFound,
	)
}
func TestWalkNotFound(t *testing.T)  { testWalkNotFound(t).Walk() }
func TestWalkRNotFound(t *testing.T) { testWalkNotFound(t).WalkR() }

func TestWalkNotFoundMaskError(t *testing.T) {
	// this doesn't work for WalkR
	newListDirs(t, nil, true,
		listResults{
			"": {err: ErrorDirNotFound},
		},
		errorMap{
			"": nil,
		},
		nil,
	).Walk()
}

func TestWalkNotFoundSkipkError(t *testing.T) {
	// this doesn't work for WalkR
	newListDirs(t, nil, true,
		listResults{
			"": {err: ErrorDirNotFound},
		},
		errorMap{
			"": ErrorSkipDir,
		},
		nil,
	).Walk()
}

func testWalkLevels(t *testing.T, maxLevel int) *listDirs {
	da := newDir("a")
	oA := mockObject("A")
	db := newDir("a/b")
	oB := mockObject("a/B")
	dc := newDir("a/b/c")
	oC := mockObject("a/b/C")
	dd := newDir("a/b/c/d")
	oD := mockObject("a/b/c/D")
	return newListDirs(t, nil, false,
		listResults{
			"":        {entries: DirEntries{oA, da}, err: nil},
			"a":       {entries: DirEntries{oB, db}, err: nil},
			"a/b":     {entries: DirEntries{oC, dc}, err: nil},
			"a/b/c":   {entries: DirEntries{oD, dd}, err: nil},
			"a/b/c/d": {entries: DirEntries{}, err: nil},
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
	entries, err := walkNDirTree(nil, "", ls.includeAll, ls.maxLevel, ls.ListDir)
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
	da := newDir("a")
	oA := mockObject("A")
	return newListDirs(t, nil, false,
		listResults{
			"": {entries: DirEntries{oA, da}, err: nil},
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
	da := newDir("a")
	oA := mockObject("A")
	db := newDir("a/b")
	oB := mockObject("a/B")
	return newListDirs(t, nil, false,
		listResults{
			"":  {entries: DirEntries{oA, da}, err: nil},
			"a": {entries: DirEntries{oB, db}, err: nil},
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
	da := newDir("a")
	db := newDir("a/b")
	dc := newDir("a/b/c")
	return newListDirs(t, nil, false,
		listResults{
			"":    {entries: DirEntries{da}, err: nil},
			"a":   {entries: DirEntries{db}, err: nil},
			"a/b": {entries: DirEntries{dc}, err: nil},
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

func testWalkErrors(t *testing.T) *listDirs {
	lr := listResults{}
	em := errorMap{}
	de := make(DirEntries, 10)
	for i := range de {
		path := string('0' + i)
		de[i] = newDir(path)
		lr[path] = listResult{entries: nil, err: ErrorDirNotFound}
		em[path] = ErrorDirNotFound
	}
	lr[""] = listResult{entries: de, err: nil}
	em[""] = nil
	return newListDirs(t, nil, true,
		lr,
		em,
		ErrorDirNotFound,
	).NoCheckMaps()
}
func TestWalkErrors(t *testing.T)  { testWalkErrors(t).Walk() }
func TestWalkRErrors(t *testing.T) { testWalkErrors(t).WalkR() }

var errorBoom = errors.New("boom")

func makeTree(level int, terminalErrors bool) (listResults, errorMap) {
	lr := listResults{}
	em := errorMap{}
	var fill func(path string, level int)
	fill = func(path string, level int) {
		de := DirEntries{}
		if level > 0 {
			for _, a := range "0123456789" {
				subPath := string(a)
				if path != "" {
					subPath = path + "/" + subPath
				}
				de = append(de, newDir(subPath))
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
func makeListRCallback(entries DirEntries, err error) ListRFn {
	return func(dir string, callback ListRCallback) error {
		if err == nil {
			err = callback(entries)
		}
		return err
	}
}

func TestWalkRDirTree(t *testing.T) {
	for _, test := range []struct {
		entries DirEntries
		want    string
		err     error
		root    string
		level   int
	}{
		{DirEntries{}, "/\n", nil, "", -1},
		{DirEntries{mockObject("a")}, `/
  a
`, nil, "", -1},
		{DirEntries{mockObject("a/b")}, `/
  a/
a/
  b
`, nil, "", -1},
		{DirEntries{mockObject("a/b/c/d")}, `/
  a/
a/
  b/
a/b/
  c/
a/b/c/
  d
`, nil, "", -1},
		{DirEntries{mockObject("a")}, "", errorBoom, "", -1},
		{DirEntries{
			mockObject("0/1/2/3"),
			mockObject("4/5/6/7"),
			mockObject("8/9/a/b"),
			mockObject("c/d/e/f"),
			mockObject("g/h/i/j"),
			mockObject("k/l/m/n"),
			mockObject("o/p/q/r"),
			mockObject("s/t/u/v"),
			mockObject("w/x/y/z"),
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
		{DirEntries{
			mockObject("a/b/c/d/e/f1"),
			mockObject("a/b/c/d/e/f2"),
			mockObject("a/b/c/d/e/f3"),
		}, `a/b/c/
  d/
a/b/c/d/
  e/
a/b/c/d/e/
  f1
  f2
  f3
`, nil, "a/b/c", -1},
		{DirEntries{
			mockObject("A"),
			mockObject("a/B"),
			mockObject("a/b/C"),
			mockObject("a/b/c/D"),
			mockObject("a/b/c/d/E"),
		}, `/
  A
  a/
a/
  B
  b/
`, nil, "", 2},
		{DirEntries{
			mockObject("a/b/c"),
			mockObject("a/b/c/d/e"),
		}, `/
  a/
a/
  b/
`, nil, "", 2},
	} {
		r, err := walkRDirTree(nil, test.root, true, test.level, makeListRCallback(test.entries, test.err))
		assert.Equal(t, test.err, err, fmt.Sprintf("%+v", test))
		assert.Equal(t, test.want, r.String(), fmt.Sprintf("%+v", test))
	}
}
