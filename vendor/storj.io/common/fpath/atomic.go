// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package fpath

import (
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
)

// AtomicWriteFile is a helper to atomically write the data to the outfile.
func AtomicWriteFile(outfile string, data []byte, _ os.FileMode) (err error) {
	// TODO: provide better atomicity guarantees, like fsyncing the parent
	// directory and, on windows, using MoveFileEx with MOVEFILE_WRITE_THROUGH.

	fh, err := os.CreateTemp(filepath.Dir(outfile), filepath.Base(outfile))
	if err != nil {
		return errs.Wrap(err)
	}
	needsClose, needsRemove := true, true

	defer func() {
		if needsClose {
			err = errs.Combine(err, errs.Wrap(fh.Close()))
		}
		if needsRemove {
			err = errs.Combine(err, errs.Wrap(os.Remove(fh.Name())))
		}
	}()

	if _, err := fh.Write(data); err != nil {
		return errs.Wrap(err)
	}

	needsClose = false
	if err := fh.Close(); err != nil {
		return errs.Wrap(err)
	}

	if err := os.Rename(fh.Name(), outfile); err != nil {
		return errs.Wrap(err)
	}
	needsRemove = false

	return nil
}
