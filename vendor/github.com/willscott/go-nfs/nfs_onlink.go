package nfs

import (
	"context"
	"os"
)

var linkErrorBody = [12]byte{}

// Backing billy.FS doesn't support hard links
func onLink(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = errFormatterWithBody(linkErrorBody[:])
	return &NFSStatusError{NFSStatusNotSupp, os.ErrPermission}
}
