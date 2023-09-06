package hdfs

import (
	"os"
	"path/filepath"
	"sort"
)

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root. All errors that arise visiting files
// and directories are filtered by walkFn. The files are walked in lexical
// order, which makes the output deterministic but means that for very large
// directories Walk can be inefficient. Walk does not follow symbolic links.
func (c *Client) Walk(root string, walkFn filepath.WalkFunc) error {
	return c.walk(root, walkFn)
}

func (c *Client) walk(path string, walkFn filepath.WalkFunc) error {
	file, err := c.Open(path)
	var info os.FileInfo
	if file != nil {
		info = file.Stat()
	}

	err = walkFn(path, info, err)
	if err != nil {
		if info != nil && info.IsDir() && err == filepath.SkipDir {
			return nil
		}

		return err
	}

	if info == nil || !info.IsDir() {
		return nil
	}

	names, err := file.Readdirnames(0)
	if err != nil {
		return walkFn(path, info, err)
	}

	sort.Strings(names)
	for _, name := range names {
		err = c.walk(filepath.ToSlash(filepath.Join(path, name)), walkFn)
		if err != nil {
			return err
		}
	}

	return nil
}
