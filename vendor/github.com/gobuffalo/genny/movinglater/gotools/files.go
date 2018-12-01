package gotools

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
)

func GoFiles(dir string) ([]string, error) {
	var files []string

	pwd, err := os.Getwd()
	if err != nil {
		return files, errors.WithStack(err)
	}
	if dir == "" {
		dir = pwd
	}

	err = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		path = strings.TrimPrefix(path, pwd+"/")
		if strings.Contains(path, ".git") || strings.Contains(path, "node_modules") || strings.Contains(path, "vendor"+string(os.PathSeparator)) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) == ".go" {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}
