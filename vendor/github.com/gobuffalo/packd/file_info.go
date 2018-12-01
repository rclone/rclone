package packd

import (
	"os"
	"time"
)

var _ os.FileInfo = fileInfo{}

type fileInfo struct {
	Path     string
	Contents []byte
	size     int64
	modTime  time.Time
	isDir    bool
}

func (f fileInfo) Name() string {
	return f.Path
}

func (f fileInfo) Size() int64 {
	return f.size
}

func (f fileInfo) Mode() os.FileMode {
	return 0444
}

func (f fileInfo) ModTime() time.Time {
	return f.modTime
}

func (f fileInfo) IsDir() bool {
	return f.isDir
}

func (f fileInfo) Sys() interface{} {
	return nil
}
