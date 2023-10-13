package nfs

import (
	"context"
)

func onRmDir(ctx context.Context, w *response, userHandle Handler) error {
	return onRemove(ctx, w, userHandle)
}
