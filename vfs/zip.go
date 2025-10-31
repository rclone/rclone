package vfs

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/rclone/rclone/fs"
)

// CreateZip creates a zip file from a vfs.Dir writing it to w
func CreateZip(ctx context.Context, dir *Dir, w io.Writer) (err error) {
	zipWriter := zip.NewWriter(w)
	defer fs.CheckClose(zipWriter, &err)
	var walk func(dir *Dir, root string) error
	walk = func(dir *Dir, root string) error {
		nodes, err := dir.ReadDirAll()
		if err != nil {
			return fmt.Errorf("create zip directory read: %w", err)
		}
		for _, node := range nodes {
			switch e := node.(type) {
			case *File:
				in, err := e.Open(os.O_RDONLY)
				if err != nil {
					return fmt.Errorf("create zip open file: %w", err)
				}
				header := &zip.FileHeader{
					Name:     root + e.Name(),
					Method:   zip.Deflate,
					Modified: e.ModTime(),
				}
				fileWriter, err := zipWriter.CreateHeader(header)
				if err != nil {
					fs.CheckClose(in, &err)
					return fmt.Errorf("create zip file header: %w", err)
				}
				_, err = io.Copy(fileWriter, in)
				if err != nil {
					fs.CheckClose(in, &err)
					return fmt.Errorf("create zip copy: %w", err)
				}
				fs.CheckClose(in, &err)
			case *Dir:
				name := root + e.Path()
				if name != "" && name[len(name)-1] != '/' {
					name += "/"
				}
				header := &zip.FileHeader{
					Name:     name,
					Method:   zip.Store,
					Modified: e.ModTime(),
				}
				_, err := zipWriter.CreateHeader(header)
				if err != nil {
					return fmt.Errorf("create zip directory header: %w", err)
				}
				err = walk(e, name)
				if err != nil {
					return err
				}
			}
		}
		return nil
	}
	err = walk(dir, "")
	if err != nil {
		return err
	}
	return nil
}
