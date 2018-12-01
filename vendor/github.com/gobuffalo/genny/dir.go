package genny

import "os"

var _ File = Dir{}

type Dir struct {
	File
	Perm os.FileMode
}

func NewDir(path string, perm os.FileMode) File {
	f := NewFileS(path, path)
	return Dir{
		File: f,
		Perm: perm,
	}
}
