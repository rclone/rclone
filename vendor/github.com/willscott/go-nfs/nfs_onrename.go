package nfs

import (
	"bytes"
	"context"
	"os"

	"github.com/go-git/go-billy/v5"
	"github.com/willscott/go-nfs-client/nfs/xdr"
)

var doubleWccErrorBody = [16]byte{}

func onRename(ctx context.Context, w *response, userHandle Handler) error {
	w.errorFmt = errFormatterWithBody(doubleWccErrorBody[:])
	from := DirOpArg{}
	err := xdr.Read(w.req.Body, &from)
	if err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}
	fs, fromPath, err := userHandle.FromHandle(from.Handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}

	to := DirOpArg{}
	if err = xdr.Read(w.req.Body, &to); err != nil {
		return &NFSStatusError{NFSStatusInval, err}
	}
	fs2, toPath, err := userHandle.FromHandle(to.Handle)
	if err != nil {
		return &NFSStatusError{NFSStatusStale, err}
	}
	if fs != fs2 {
		return &NFSStatusError{NFSStatusNotSupp, os.ErrPermission}
	}

	if !billy.CapabilityCheck(fs, billy.WriteCapability) {
		return &NFSStatusError{NFSStatusROFS, os.ErrPermission}
	}

	if len(string(from.Filename)) > PathNameMax || len(string(to.Filename)) > PathNameMax {
		return &NFSStatusError{NFSStatusNameTooLong, os.ErrInvalid}
	}

	fromDirInfo, err := fs.Stat(fs.Join(fromPath...))
	if err != nil {
		if os.IsNotExist(err) {
			return &NFSStatusError{NFSStatusNoEnt, err}
		}
		return &NFSStatusError{NFSStatusIO, err}
	}
	if !fromDirInfo.IsDir() {
		return &NFSStatusError{NFSStatusNotDir, nil}
	}
	preCacheData := ToFileAttribute(fromDirInfo).AsCache()

	toDirInfo, err := fs.Stat(fs.Join(toPath...))
	if err != nil {
		if os.IsNotExist(err) {
			return &NFSStatusError{NFSStatusNoEnt, err}
		}
		return &NFSStatusError{NFSStatusIO, err}
	}
	if !toDirInfo.IsDir() {
		return &NFSStatusError{NFSStatusNotDir, nil}
	}
	preDestData := ToFileAttribute(toDirInfo).AsCache()

	fromLoc := fs.Join(append(fromPath, string(from.Filename))...)
	toLoc := fs.Join(append(toPath, string(to.Filename))...)

	err = fs.Rename(fromLoc, toLoc)
	if err != nil {
		if os.IsNotExist(err) {
			return &NFSStatusError{NFSStatusNoEnt, err}
		}
		if os.IsPermission(err) {
			return &NFSStatusError{NFSStatusAccess, err}
		}
		return &NFSStatusError{NFSStatusIO, err}
	}

	writer := bytes.NewBuffer([]byte{})
	if err := xdr.Write(writer, uint32(NFSStatusOk)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := WriteWcc(writer, preCacheData, tryStat(fs, fromPath)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	if err := WriteWcc(writer, preDestData, tryStat(fs, toPath)); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}

	if err := w.Write(writer.Bytes()); err != nil {
		return &NFSStatusError{NFSStatusServerFault, err}
	}
	return nil
}
