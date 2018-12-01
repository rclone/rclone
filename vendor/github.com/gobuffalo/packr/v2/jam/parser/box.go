package parser

import (
	"encoding/json"
	"os"
	"strings"
)

// Box found while parsing a file
type Box struct {
	Name       string // name of the box
	Path       string // relative path of folder NewBox("./templates")
	AbsPath    string // absolute path of Path
	Package    string // the package name the box was found in
	PWD        string // the PWD when the parser was run
	PackageDir string // the absolute path of the package where the box was found
}

type Boxes []*Box

// String - json returned
func (b Box) String() string {
	x, _ := json.Marshal(b)
	return string(x)
}

// NewBox stub from the name and the path provided
func NewBox(name string, path string) *Box {
	if len(name) == 0 {
		name = path
	}
	name = strings.Replace(name, "\"", "", -1)
	pwd, _ := os.Getwd()
	box := &Box{
		Name: name,
		Path: path,
		PWD:  pwd,
	}
	return box
}
