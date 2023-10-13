package nfs

import (
	"bytes"
	"context"

	"github.com/go-git/go-billy/v5"
	"github.com/willscott/go-nfs-client/nfs/xdr"
)

func onFSStat(ctx context.Context, w *response, userHandle Handler) error {
	roothandle, err := xdr.ReadOpaque(w.req.Body)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}
	fs, path, err := userHandle.FromHandle(roothandle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}

	defaults := FSStat{
		TotalSize:      1 << 62,
		FreeSize:       1 << 62,
		AvailableSize:  1 << 62,
		TotalFiles:     1 << 62,
		FreeFiles:      1 << 62,
		AvailableFiles: 1 << 62,
		CacheHint:      0,
	}
	if !billy.CapabilityCheck(fs, billy.WriteCapability) {
		defaults.AvailableFiles = 0
		defaults.AvailableSize = 0
	}

	err = userHandle.FSStat(ctx, fs, &defaults)
	if err != nil {
		if _, ok := err.(*NFSStatusError); ok {
			return err
		}
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := WritePostOpAttrs(writer, tryStat(fs, path)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := xdr.Write(writer, defaults); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}
