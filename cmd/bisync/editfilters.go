package bisync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/rclone/rclone/cmd/genfilters"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/lib/terminal"
)

// editFiltersFn and confirmFn can be replaced in tests
var (
	editFiltersFn = genfilters.Run
	confirmFn     = config.Confirm
)

// editFilters lets the user edit --filters-file with genfilters. It
// returns whether to go on with the run, which must be a resync if the
// filters changed.
func (opt *Options) editFilters(ctx context.Context, fs1, fs2 fs.Fs) (run bool, err error) {
	if opt.EditFilters && opt.EditFiltersPath2 {
		return false, errors.New("--edit-filters and --edit-filters-path2 can't be used together")
	}
	if opt.FiltersFile == "" {
		return false, errors.New("--edit-filters needs --filters-file")
	}
	f := fs1
	if opt.EditFiltersPath2 {
		f = fs2
	}
	before, err := os.ReadFile(opt.FiltersFile)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	if err = editFiltersFn(ctx, f, genfilters.Options{Edit: opt.FiltersFile}); err != nil {
		return false, err
	}
	after, err := os.ReadFile(opt.FiltersFile)
	if err != nil {
		return false, err
	}
	if bytes.Equal(before, after) || opt.Resync {
		return true, nil
	}

	operations.StdoutMutex.Lock()
	fmt.Println(Color(terminal.YellowFg, "The filters changed, so bisync needs a --resync. Do one now?"))
	resync := confirmFn(false)
	operations.StdoutMutex.Unlock()
	if !resync {
		fs.Logf(nil, "Not syncing. Run bisync with --resync to use the new filters.")
		return false, nil
	}
	opt.Resync = true
	return true, nil
}
