package fstestutil

import (
	"fmt"
	"io/ioutil"
	"os"
)

// FileInfoCheck is a function that validates an os.FileInfo according
// to some criteria.
type FileInfoCheck func(fi os.FileInfo) error

type checkDirError struct {
	missing map[string]struct{}
	extra   map[string]os.FileMode
}

func (e *checkDirError) Error() string {
	return fmt.Sprintf("wrong directory contents: missing %v, extra %v", e.missing, e.extra)
}

// CheckDir checks the contents of the directory at path, making sure
// every directory entry listed in want is present. If the check is
// not nil, it must also pass.
//
// If want contains the impossible filename "", unexpected files are
// checked with that. If the key is not in want, unexpected files are
// an error.
//
// Missing entries, that are listed in want but not seen, are an
// error.
func CheckDir(path string, want map[string]FileInfoCheck) error {
	problems := &checkDirError{
		missing: make(map[string]struct{}, len(want)),
		extra:   make(map[string]os.FileMode),
	}
	for k := range want {
		if k == "" {
			continue
		}
		problems.missing[k] = struct{}{}
	}

	fis, err := ioutil.ReadDir(path)
	if err != nil {
		return fmt.Errorf("cannot read directory: %v", err)
	}

	for _, fi := range fis {
		check, ok := want[fi.Name()]
		if !ok {
			check, ok = want[""]
		}
		if !ok {
			problems.extra[fi.Name()] = fi.Mode()
			continue
		}
		delete(problems.missing, fi.Name())
		if check != nil {
			if err := check(fi); err != nil {
				return fmt.Errorf("check failed: %v: %v", fi.Name(), err)
			}
		}
	}

	if len(problems.missing) > 0 || len(problems.extra) > 0 {
		return problems
	}
	return nil
}
