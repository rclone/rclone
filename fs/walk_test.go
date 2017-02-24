package fs

import (
	"sync"
	"testing"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
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

func newDir(name string) *Dir {
	return &Dir{Name: name}
}

func TestWalkEmpty(t *testing.T) {
	newListDirs(t, nil, false,
		listResults{
			"": {entries: DirEntries{}, err: nil},
		},
		errorMap{
			"": nil,
		},
		nil,
	).Walk()
}

func TestWalkEmptySkip(t *testing.T) {
	newListDirs(t, nil, true,
		listResults{
			"": {entries: DirEntries{}, err: nil},
		},
		errorMap{
			"": ErrorSkipDir,
		},
		nil,
	).Walk()
}

func TestWalkNotFound(t *testing.T) {
	newListDirs(t, nil, true,
		listResults{
			"": {err: ErrorDirNotFound},
		},
		errorMap{
			"": ErrorDirNotFound,
		},
		ErrorDirNotFound,
	).Walk()
}

func TestWalkNotFoundMaskError(t *testing.T) {
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

func testWalkLevels(t *testing.T, maxLevel int) {
	da := newDir("a")
	db := newDir("a/b")
	dc := newDir("a/b/c")
	dd := newDir("a/b/c/d")
	newListDirs(t, nil, false,
		listResults{
			"":        {entries: DirEntries{da}, err: nil},
			"a":       {entries: DirEntries{db}, err: nil},
			"a/b":     {entries: DirEntries{dc}, err: nil},
			"a/b/c":   {entries: DirEntries{dd}, err: nil},
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
	).SetLevel(maxLevel).Walk()
}

func TestWalkLevels(t *testing.T) {
	testWalkLevels(t, -1)
}

func TestWalkLevelsNoRecursive10(t *testing.T) {
	testWalkLevels(t, 10)
}

func TestWalkLevelsNoRecursive(t *testing.T) {
	da := newDir("a")
	newListDirs(t, nil, false,
		listResults{
			"": {entries: DirEntries{da}, err: nil},
		},
		errorMap{
			"": nil,
		},
		nil,
	).SetLevel(1).Walk()
}

func TestWalkLevels2(t *testing.T) {
	da := newDir("a")
	db := newDir("a/b")
	newListDirs(t, nil, false,
		listResults{
			"":  {entries: DirEntries{da}, err: nil},
			"a": {entries: DirEntries{db}, err: nil},
		},
		errorMap{
			"":  nil,
			"a": nil,
		},
		nil,
	).SetLevel(2).Walk()
}

func TestWalkSkip(t *testing.T) {
	da := newDir("a")
	db := newDir("a/b")
	dc := newDir("a/b/c")
	newListDirs(t, nil, false,
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
	).Walk()
}

func TestWalkErrors(t *testing.T) {
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
	newListDirs(t, nil, true,
		lr,
		em,
		ErrorDirNotFound,
	).NoCheckMaps().Walk()
}

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

func TestWalkMulti(t *testing.T) {
	lr, em := makeTree(3, false)
	newListDirs(t, nil, true,
		lr,
		em,
		nil,
	).Walk()
}

func TestWalkMultiErrors(t *testing.T) {
	lr, em := makeTree(3, true)
	newListDirs(t, nil, true,
		lr,
		em,
		errorBoom,
	).NoCheckMaps().Walk()
}
