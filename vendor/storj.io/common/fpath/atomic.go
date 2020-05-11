// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
)

// AtomicWriteFile is a helper to atomically write the data to the outfile.
func AtomicWriteFile(outfile string, data []byte, mode os.FileMode) (err error) {
	// TODO: provide better atomicity guarantees, like fsyncing the parent
	// directory and, on windows, using MoveFileEx with MOVEFILE_WRITE_THROUGH.

	fh, err := ioutil.TempFile(filepath.Dir(outfile), filepath.Base(outfile))
	if err != nil {
		return errs.Wrap(err)
	}
	defer func() {
		if err != nil {
			err = errs.Combine(err, fh.Close())
			err = errs.Combine(err, os.Remove(fh.Name()))
		}
	}()
	if _, err := fh.Write(data); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Sync(); err != nil {
		return errs.Wrap(err)
	}
	if err := fh.Close(); err != nil {
		return errs.Wrap(err)
	}
	if err := os.Rename(fh.Name(), outfile); err != nil {
		return errs.Wrap(err)
	}
	return nil
}
