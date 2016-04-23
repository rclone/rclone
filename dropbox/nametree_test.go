package dropbox

import (
	"testing"

	"github.com/ncw/rclone/fs"
	dropboxapi "github.com/stacktic/dropbox"
	"github.com/stretchr/testify/assert"
)

func TestPutCaseCorrectDirectoryName(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("a/b", "C")

	assert.Equal(t, "", tree.CaseCorrectName, "Root CaseCorrectName should be empty")

	a := tree.Directories["a"]
	assert.Equal(t, "", a.CaseCorrectName, "CaseCorrectName at 'a' should be empty")

	b := a.Directories["b"]
	assert.Equal(t, "", b.CaseCorrectName, "CaseCorrectName at 'a/b' should be empty")

	c := b.Directories["c"]
	assert.Equal(t, "C", c.CaseCorrectName, "CaseCorrectName at 'a/b/c' should be 'C'")

	assert.Equal(t, errors, fs.Stats.GetErrors(), "No errors should be reported")
}

func TestPutCaseCorrectPath(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectPath("A/b/C")

	assert.Equal(t, "", tree.CaseCorrectName, "Root CaseCorrectName should be empty")

	a := tree.Directories["a"]
	assert.Equal(t, "A", a.CaseCorrectName, "CaseCorrectName at 'a' should be 'A'")

	b := a.Directories["b"]
	assert.Equal(t, "b", b.CaseCorrectName, "CaseCorrectName at 'a/b' should be 'b'")

	c := b.Directories["c"]
	assert.Equal(t, "C", c.CaseCorrectName, "CaseCorrectName at 'a/b/c' should be 'C'")

	assert.Equal(t, errors, fs.Stats.GetErrors(), "No errors should be reported")
}

func TestPutCaseCorrectDirectoryNameEmptyComponent(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("/a", "C")
	tree.PutCaseCorrectDirectoryName("b/", "C")
	tree.PutCaseCorrectDirectoryName("a//b", "C")

	assert.True(t, fs.Stats.GetErrors() == errors+3, "3 errors should be reported")
}

func TestPutCaseCorrectDirectoryNameEmptyParent(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("", "C")

	c := tree.Directories["c"]
	assert.True(t, c.CaseCorrectName == "C", "CaseCorrectName at 'c' should be 'C'")

	assert.True(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestGetPathWithCorrectCase(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutCaseCorrectDirectoryName("a", "C")
	assert.True(t, tree.GetPathWithCorrectCase("a/c") == nil, "Path for 'a' should not be available")

	tree.PutCaseCorrectDirectoryName("", "A")
	assert.True(t, *tree.GetPathWithCorrectCase("a/c") == "/A/C", "Path for 'a/c' should be '/A/C'")

	assert.True(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutAndWalk(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutFile("a", "F", &dropboxapi.Entry{Path: "xxx"})
	tree.PutCaseCorrectDirectoryName("", "A")

	numCalled := 0
	walkFunc := func(caseCorrectFilePath string, entry *dropboxapi.Entry) error {
		assert.True(t, caseCorrectFilePath == "A/F", "caseCorrectFilePath should be A/F, not "+caseCorrectFilePath)
		assert.True(t, entry.Path == "xxx", "entry.Path should be xxx")
		numCalled++
		return nil
	}
	err := tree.WalkFiles("", walkFunc)
	assert.True(t, err == nil, "No error should be returned")
	assert.True(t, numCalled == 1, "walk func should be called only once")
	assert.True(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
}

func TestPutAndWalkWithPrefix(t *testing.T) {
	errors := fs.Stats.GetErrors()

	tree := newNameTree()
	tree.PutFile("a", "F", &dropboxapi.Entry{Path: "xxx"})
	tree.PutCaseCorrectDirectoryName("", "A")

	numCalled := 0
	walkFunc := func(caseCorrectFilePath string, entry *dropboxapi.Entry) error {
		assert.True(t, caseCorrectFilePath == "A/F", "caseCorrectFilePath should be A/F, not "+caseCorrectFilePath)
		assert.True(t, entry.Path == "xxx", "entry.Path should be xxx")
		numCalled++
		return nil
	}
	err := tree.WalkFiles("A", walkFunc)
	assert.True(t, err == nil, "No error should be returned")
	assert.True(t, numCalled == 1, "walk func should be called only once")
	assert.True(t, fs.Stats.GetErrors() == errors, "No errors should be reported")
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
	assert.True(t, err == nil, "No error should be returned")
	assert.True(t, fs.Stats.GetErrors() == errors+1, "One error should be reported")
}
