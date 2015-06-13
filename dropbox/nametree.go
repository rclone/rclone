package dropbox

import (
	"strings"
	"bytes"
	"fmt"
	"github.com/stacktic/dropbox"
	"github.com/ncw/rclone/fs"
)

type NameTreeNode struct {
	// Map from lowercase directory name to tree node
	Directories     map[string]*NameTreeNode

	// Map from file name (case sensitive) to dropbox entry
	Files           map[string]*dropbox.Entry

	// Empty string if exact case is unknown or root node
	CaseCorrectName string
}

// ------------------------------------------------------------

func newNameTreeNode(caseCorrectName string) *NameTreeNode {
	return &NameTreeNode{
		CaseCorrectName: caseCorrectName,
		Directories: make(map[string]*NameTreeNode),
		Files: make(map[string]*dropbox.Entry),
	}
}

func NewNameTree() *NameTreeNode {
	return newNameTreeNode("")
}

func (tree *NameTreeNode) String() string {
	if len(tree.CaseCorrectName) == 0 {
		return "NameTreeNode/<root>"
	} else {
		return fmt.Sprintf("NameTreeNode/%q", tree.CaseCorrectName)
	}
}

func (tree *NameTreeNode) getTreeNode(path string) *NameTreeNode {
	if len(path) == 0 {
		// no lookup required, just return root
		return tree
	}

	current := tree
	for _, component := range strings.Split(path, "/") {
		if len(component) == 0 {
			fs.Stats.Error()
			fs.Log(tree, "getTreeNode: path component is empty (full path %q)", path)
			return nil
		}

		lowercase := strings.ToLower(component)

		lookup := current.Directories[lowercase]
		if lookup == nil {
			lookup = newNameTreeNode("")
			current.Directories[lowercase] = lookup
		}

		current = lookup
	}

	return current
}

func (tree *NameTreeNode) PutCaseCorrectDirectoryName(parentPath string, caseCorrectDirectoryName string) {
	if len(caseCorrectDirectoryName) == 0 {
		fs.Stats.Error()
		fs.Log(tree, "PutCaseCorrectDirectoryName: empty caseCorrectDirectoryName is not allowed (parentPath: %q)", parentPath)
		return
	}

	node := tree.getTreeNode(parentPath)
	if node == nil {
		return
	}

	lowerCaseDirectoryName := strings.ToLower(caseCorrectDirectoryName)
	directory := node.Directories[lowerCaseDirectoryName]
	if directory == nil {
		directory = newNameTreeNode(caseCorrectDirectoryName)
		node.Directories[lowerCaseDirectoryName] = directory
	} else {
		if len(directory.CaseCorrectName) > 0 {
			fs.Stats.Error()
			fs.Log(tree, "PutCaseCorrectDirectoryName: directory %q is already exists under parent path %q", caseCorrectDirectoryName, parentPath)
			return
		}

		directory.CaseCorrectName = caseCorrectDirectoryName
	}
}

func (tree *NameTreeNode) PutFile(parentPath string, caseCorrectFileName string, dropboxEntry *dropbox.Entry) {
	node := tree.getTreeNode(parentPath)
	if node == nil {
		return
	}

	if node.Files[caseCorrectFileName] != nil {
		fs.Stats.Error()
		fs.Log(tree, "PutFile: file %q is already exists at %q", caseCorrectFileName, parentPath)
		return
	}

	node.Files[caseCorrectFileName] = dropboxEntry
}

func (tree *NameTreeNode) GetPathWithCorrectCase(path string) *string {
	if path == "" {
		empty := ""
		return &empty
	}

	var result bytes.Buffer

	current := tree
	for _, component := range strings.Split(path, "/") {
		if component == "" {
			fs.Stats.Error()
			fs.Log(tree, "GetPathWithCorrectCase: path component is empty (full path %q)", path)
			return nil
		}

		lowercase := strings.ToLower(component)

		current = current.Directories[lowercase]
		if current == nil || current.CaseCorrectName == "" {
			return nil
		}

		result.WriteString("/")
		result.WriteString(current.CaseCorrectName)
	}

	resultString := result.String()
	return &resultString
}

type NameTreeFileWalkFunc func(caseCorrectFilePath string, entry *dropbox.Entry)

func (tree *NameTreeNode) walkFilesRec(currentPath string, walkFunc NameTreeFileWalkFunc) {
	var prefix string
	if (currentPath == "") {
		prefix = ""
	} else {
		prefix = currentPath + "/"
	}

	for name, entry := range tree.Files {
		walkFunc(prefix + name, entry)
	}

	for lowerCaseName, directory := range tree.Directories {
		caseCorrectName := directory.CaseCorrectName
		if caseCorrectName == "" {
			fs.Stats.Error()
			fs.Log(tree, "WalkFiles: exact name of the directory %q is unknown (parent path: %q)", lowerCaseName, currentPath)
			continue
		}

		directory.walkFilesRec(prefix + caseCorrectName, walkFunc)
	}
}

func (tree *NameTreeNode) WalkFiles(rootPath string, walkFunc NameTreeFileWalkFunc) {
	node := tree.getTreeNode(rootPath)
	if node == nil {
		return
	}

	node.walkFilesRec(rootPath, walkFunc)
}
