package nfs

import (
	"bytes"
	"context"
	"os"

	"github.com/willscott/go-nfs-client/nfs/xdr"
)

func onGetAttr(ctx context.Context, w *response, userHandle Handler) error {
	handle, err := xdr.ReadOpaque(w.req.Body)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}

	fs, path, err := userHandle.FromHandle(handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}

	info, err := fs.Stat(fs.Join(path...))
	if err != nil {
		if os.IsNotExist(err) {
			return &NFSStatusError{NFSStatusNoEnt, err}
		}
		return &NFSStatusError{NFSStatusIO, err}
	}
	attr := ToFileAttribute(info)

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := xdr.Write(writer, attr); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}
