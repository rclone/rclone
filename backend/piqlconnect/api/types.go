package api

import (
	"context"
	"path"
	"time"

	"github.com/rclone/rclone/fs"
)

type Package struct {
	Id        string `json:"id"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	UpdatedAt string `json:"updatedAt"`
}

type File struct {
	fs        fs.Info
	Path      string `json:"path"`
	UpdatedAt string `json:"updatedAt"`
	FileSize  int64  `json:"size"`
}

func (file File) Fs() fs.Info {
	return file.fs
}

func (file *File) SetFs(fs fs.Info) {
	file.fs = fs
}

func (file File) String() string {
	return path.Base(file.Path)
}

func (file File) Remote() string {
	return file.Path
}

func (file File) ModTime(ctx context.Context) time.Time {
	mtime, err := time.Parse(time.RFC3339, file.UpdatedAt)
	if err != nil {
		panic("Failed to parse time")
	}
	return mtime
}

func (file File) Size() int64 {
	return file.FileSize
}
