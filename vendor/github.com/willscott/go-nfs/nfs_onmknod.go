package nfs

import (
	"context"
	"os"
)

// Backing billy.FS doesn't support creation of
// char, block, socket, or fifo pipe nodes
func onMknod(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = wccDataErrorFormatter
	return &NFSStatusError{NFSStatusNotSupp, os.ErrPermission}
}
