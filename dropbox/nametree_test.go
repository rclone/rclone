package dropbox

import (
	"testing"

	"github.com/ncw/rclone/fs"
	dropboxapi "github.com/stacktic/dropbox"
)

func assert(t *testing.T, shouldBeTrue bool, failMessage string) {
	if !shouldBeTrue {
		t.Fatal(failMessage)
	}
}

func TestPutCaseCorrectDirectoryName(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("a/b", "C")

	assert(t, tree.CaseCorrectName == "", "Root CaseCorrectName should be empty")

	a := tree.Directories["a"]
	assert(t, a.CaseCorrectName == "", "CaseCorrectName at 'a' should be empty")

	b := a.Directories["b"]
	assert(t, b.CaseCorrectName == "", "CaseCorrectName at 'a/b' should be empty")

	c := b.Directories["c"]
	assert(t, c.CaseCorrectName == "C", "CaseCorrectName at 'a/b/c' should be 'C'")

	assert(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutCaseCorrectDirectoryNameEmptyComponent(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("/a", "C")
	tree.PutCaseCorrectDirectoryName("b/", "C")
	tree.PutCaseCorrectDirectoryName("a//b", "C")

	assert(t, fs.Stats.GetErrors() == errors+3, "3 errors should be reported")
}

func TestPutCaseCorrectDirectoryNameEmptyParent(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("", "C")

	c := tree.Directories["c"]
	assert(t, c.CaseCorrectName == "C", "CaseCorrectName at 'c' should be 'C'")

	assert(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestGetPathWithCorrectCase(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("a", "C")
	assert(t, tree.GetPathWithCorrectCase("a/c") == nil, "Path for 'a' should not be available")

	tree.PutCaseCorrectDirectoryName("", "A")
	assert(t, *tree.GetPathWithCorrectCase("a/c") == "/A/C", "Path for 'a/c' should be '/A/C'")

	assert(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutAndWalk(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutFile("a", "F", &dropboxapi.Entry{Path: "xxx"})
	tree.PutCaseCorrectDirectoryName("", "A")

	numCalled := 0
	walkFunc := func(caseCorrectFilePath string, entry *dropboxapi.Entry) error {
		assert(t, caseCorrectFilePath == "A/F", "caseCorrectFilePath should be A/F, not "+caseCorrectFilePath)
		assert(t, entry.Path == "xxx", "entry.Path should be xxx")
		numCalled++
		return nil
	}
	err := tree.WalkFiles("", walkFunc)
	assert(t, err == nil, "No error should be returned")
	assert(t, numCalled == 1, "walk func should be called only once")
	assert(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutAndWalkWithPrefix(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutFile("a", "F", &dropboxapi.Entry{Path: "xxx"})
	tree.PutCaseCorrectDirectoryName("", "A")

	numCalled := 0
	walkFunc := func(caseCorrectFilePath string, entry *dropboxapi.Entry) error {
		assert(t, caseCorrectFilePath == "A/F", "caseCorrectFilePath should be A/F, not "+caseCorrectFilePath)
		assert(t, entry.Path == "xxx", "entry.Path should be xxx")
		numCalled++
		return nil
	}
	err := tree.WalkFiles("A", walkFunc)
	assert(t, err == nil, "No error should be returned")
	assert(t, numCalled == 1, "walk func should be called only once")
	assert(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutAndWalkIncompleteTree(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutFile("a", "F", &dropboxapi.Entry{Path: "xxx"})

	walkFunc := func(caseCorrectFilePath string, entry *dropboxapi.Entry) error {
		t.Fatal("Should not be called")
		return nil
	}
	err := tree.WalkFiles("", walkFunc)
	assert(t, err == nil, "No error should be returned")
	assert(t, fs.Stats.GetErrors() == errors+1, "One error should be reported")
}
