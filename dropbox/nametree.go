package dropbox

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ncw/rclone/fs"
	"github.com/stacktic/dropbox"
)

// FIXME Get rid of Stats.Error() counting and return errors

type nameTreeNode struct {
	// Map from lowercase directory name to tree node
	Directories map[string]*nameTreeNode

	// Map from file name (case sensitive) to dropbox entry
	Files map[string]*dropbox.Entry

	// Empty string if exact case is unknown or root node
	CaseCorrectName string
}

// ------------------------------------------------------------

func newNameTreeNode(caseCorrectName string) *nameTreeNode {
	return &nameTreeNode{
		CaseCorrectName: caseCorrectName,
		Directories:     make(map[string]*nameTreeNode),
		Files:           make(map[string]*dropbox.Entry),
	}
}

func newNameTree() *nameTreeNode {
	return newNameTreeNode("")
}

func (tree *nameTreeNode) String() string {
	if len(tree.CaseCorrectName) == 0 {
		return "nameTreeNode/<root>"
	}
	return fmt.Sprintf("nameTreeNode/%q", tree.CaseCorrectName)
}

func (tree *nameTreeNode) getTreeNode(path string) *nameTreeNode {
	if len(path) == 0 {
		// no lookup required, just return root
		return tree
	}
	current := tree
	for _, component := range strings.Split(path, "/") {
		if len(component) == 0 {
			fs.Stats.Error()
			fs.ErrorLog(tree, "getTreeNode: path component is empty (full path %q)", path)
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

// PutCaseCorrectPath puts a known good path into the nameTree
func (tree *nameTreeNode) PutCaseCorrectPath(caseCorrectPath string) {
	if len(caseCorrectPath) == 0 {
		return
	}
	current := tree
	for _, component := range strings.Split(caseCorrectPath, "/") {
		if len(component) == 0 {
			fs.Stats.Error()
			fs.ErrorLog(tree, "PutCaseCorrectPath: path component is empty (full path %q)", caseCorrectPath)
			return
		}
		lowercase := strings.ToLower(component)
		lookup := current.Directories[lowercase]
		if lookup == nil {
			lookup = newNameTreeNode(component)
			current.Directories[lowercase] = lookup
		}
		current = lookup
	}
	return
}

func (tree *nameTreeNode) PutCaseCorrectDirectoryName(parentPath string, caseCorrectDirectoryName string) {
	if len(caseCorrectDirectoryName) == 0 {
		fs.Stats.Error()
		fs.ErrorLog(tree, "PutCaseCorrectDirectoryName: empty caseCorrectDirectoryName is not allowed (parentPath: %q)", parentPath)
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
			fs.ErrorLog(tree, "PutCaseCorrectDirectoryName: directory %q is already exists under parent path %q", caseCorrectDirectoryName, parentPath)
			return
		}

		directory.CaseCorrectName = caseCorrectDirectoryName
	}
}

func (tree *nameTreeNode) PutFile(parentPath string, caseCorrectFileName string, dropboxEntry *dropbox.Entry) {
	node := tree.getTreeNode(parentPath)
	if node == nil {
		return
	}

	if node.Files[caseCorrectFileName] != nil {
		fs.Stats.Error()
		fs.ErrorLog(tree, "PutFile: file %q is already exists at %q", caseCorrectFileName, parentPath)
		return
	}

	node.Files[caseCorrectFileName] = dropboxEntry
}

func (tree *nameTreeNode) GetPathWithCorrectCase(path string) *string {
	if path == "" {
		empty := ""
		return &empty
	}

	var result bytes.Buffer

	current := tree
	for _, component := range strings.Split(path, "/") {
		if component == "" {
			fs.Stats.Error()
			fs.ErrorLog(tree, "GetPathWithCorrectCase: path component is empty (full path %q)", path)
			return nil
		}

		lowercase := strings.ToLower(component)

		current = current.Directories[lowercase]
		if current == nil || current.CaseCorrectName == "" {
			return nil
		}

		_, _ = result.WriteString("/")
		_, _ = result.WriteString(current.CaseCorrectName)
	}

	resultString := result.String()
	return &resultString
}

type nameTreeFileWalkFunc func(caseCorrectFilePath string, entry *dropbox.Entry) error

func (tree *nameTreeNode) walkFilesRec(currentPath string, walkFunc nameTreeFileWalkFunc) error {
	var prefix string
	if currentPath == "" {
		prefix = ""
	} else {
		prefix = currentPath + "/"
	}

	for name, entry := range tree.Files {
		err := walkFunc(prefix+name, entry)
		if err != nil {
			return err
		}
	}

	for lowerCaseName, directory := range tree.Directories {
		caseCorrectName := directory.CaseCorrectName
		if caseCorrectName == "" {
			fs.Stats.Error()
			fs.ErrorLog(tree, "WalkFiles: exact name of the directory %q is unknown (parent path: %q)", lowerCaseName, currentPath)
			continue
		}

		err := directory.walkFilesRec(prefix+caseCorrectName, walkFunc)
		if err != nil {
			return err
		}
	}

	return nil
}

func (tree *nameTreeNode) WalkFiles(rootPath string, walkFunc nameTreeFileWalkFunc) error {
	node := tree.getTreeNode(rootPath)
	if node == nil {
		return nil
	}

	return node.walkFilesRec(rootPath, walkFunc)
}
