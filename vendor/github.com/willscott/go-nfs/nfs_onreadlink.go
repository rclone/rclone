package nfs

import (
	"bytes"
	"context"
	"os"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

func onReadLink(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = opAttrErrorFormatter
	handle, err := xdr.ReadOpaque(w.req.Body)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}
	fs, path, err := userHandle.FromHandle(handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}

	out, err := fs.Readlink(fs.Join(path...))
	if err != nil {
		if info, err := fs.Stat(fs.Join(path...)); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				return &NFSStatusError{NFSStatusInval, err}
			}
		}
		if os.IsNotExist(err) {
			return &NFSStatusError{NFSStatusNoEnt, err}
		}

		return &NFSStatusError{NFSStatusAccess, err}
	}

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := WritePostOpAttrs(writer, tryStat(fs, path)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := xdr.Write(writer, out); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}
