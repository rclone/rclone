package batcher

import (
	"fmt"
	"time"

	"github.com/rclone/rclone/fs"
)

// FsOptions returns the batch mode fs.Options
func (opt *Options) FsOptions(extra string) []fs.Option {
	return []fs.Option{{
		Name: "batch_mode",
		Help: fmt.Sprintf(`Upload file batching sync|async|off.

This sets the batch mode used by rclone.

%sThis has 3 possible values

- off - no batching
- sync - batch uploads and check completion (default)
- async - batch upload and don't check completion

Rclone will close any outstanding batches when it exits which may make
a delay on quit.
`, extra),
		Default:  "sync",
		Advanced: true,
	}, {
		Name: "batch_size",
		Help: fmt.Sprintf(`Max number of files in upload batch.

This sets the batch size of files to upload. It has to be less than %d.

By default this is 0 which means rclone will calculate the batch size
depending on the setting of batch_mode.

- batch_mode: async - default batch_size is %d
- batch_mode: sync - default batch_size is the same as --transfers
- batch_mode: off - not in use

Rclone will close any outstanding batches when it exits which may make
a delay on quit.

Setting this is a great idea if you are uploading lots of small files
as it will make them a lot quicker. You can use --transfers 32 to
maximise throughput.
`, opt.MaxBatchSize, opt.DefaultBatchSizeAsync),
		Default:  0,
		Advanced: true,
	}, {
		Name: "batch_timeout",
		Help: fmt.Sprintf(`Max time to allow an idle upload batch before uploading.

If an upload batch is idle for more than this long then it will be
uploaded.

The default for this is 0 which means rclone will choose a sensible
default based on the batch_mode in use.

- batch_mode: async - default batch_timeout is %v
- batch_mode: sync - default batch_timeout is %v
- batch_mode: off - not in use
`, opt.DefaultTimeoutAsync, opt.DefaultTimeoutSync),
		Default:  fs.Duration(0),
		Advanced: true,
	}, {
		Name:     "batch_commit_timeout",
		Help:     `Max time to wait for a batch to finish committing`,
		Default:  fs.Duration(10 * time.Minute),
		Advanced: true,
	}}
}
