package genny

import (
	"bytes"
	"context"
	"io"
	"os"

	"github.com/gobuffalo/logger"
	"github.com/pkg/errors"
)

// DryRunner will NOT execute commands and write files
// it is NOT destructive
func DryRunner(ctx context.Context) *Runner {
	r := NewRunner(ctx)
	r.Logger = logger.New(logger.DebugLevel)
	r.FileFn = func(f File) (File, error) {
		bb := &bytes.Buffer{}
		mw := io.MultiWriter(bb, os.Stdout)
		if _, err := io.Copy(mw, f); err != nil {
			return f, errors.WithStack(err)
		}
		return NewFile(f.Name(), bb), nil
	}
	return r
}
