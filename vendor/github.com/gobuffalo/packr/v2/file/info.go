package file

import (
	"os"
	"time"
)

type info struct {
	Path     string
	Contents []byte
	size     int64
	modTime  time.Time
	isDir    bool
}

func (f info) Name() string {
	return f.Path
}

func (f info) Size() int64 {
	return f.size
}

func (f info) Mode() os.FileMode {
	return 0444
}

func (f info) ModTime() time.Time {
	return f.modTime
}

func (f info) IsDir() bool {
	return f.isDir
}

func (f info) Sys() interface{} {
	return nil
}
