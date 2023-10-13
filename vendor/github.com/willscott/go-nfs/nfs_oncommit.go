package nfs

import (
	"bytes"
	"context"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/willscott/go-nfs-client/nfs/xdr"
)

// onCommit - note this is a no-op, as we always push writes to the backing store.
func onCommit(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = wccDataErrorFormatter
	handle, err := xdr.ReadOpaque(w.req.Body)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}
	// The conn will drain the unread offset and count arguments.

	fs, path, err := userHandle.FromHandle(handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}
	if !billy.CapabilityCheck(fs, billy.WriteCapability) {
		return &NFSStatusError{NFSStatusServerFault, os.ErrPermission}
	}

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return err
	}

	// no pre-op cache data.
	if err := xdr.Write(writer, uint32(0)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := WritePostOpAttrs(writer, tryStat(fs, path)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	// write the 8 bytes of write verification.
	if err := xdr.Write(writer, w.Server.ID); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}
