package teldrive

import (
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"net/url"
	"path"
	"sort"
	"strconv"
	"sync"

	"github.com/gofrs/uuid"
	"github.com/rclone/rclone/backend/teldrive/api"
	"github.com/rclone/rclone/lib/rest"

	"github.com/rclone/rclone/fs"
)

type uploadInfo struct {
	existingChunks []api.PartFile
	uploadID       string
	channelID      int64
	encryptFile    bool
}

type objectChunkWriter struct {
	chunkSize       int64
	size            int64
	f               *Fs
	uploadID        string
	src             fs.ObjectInfo
	partsToCommitMu sync.Mutex
	partsToCommit   []api.PartFile
	existingParts   map[int]api.PartFile
	o               *Object
	totalParts      int64
	channelID       int64
	encryptFile     bool
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *objectChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (size int64, err error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}

	chunkNumber += 1

	if existing, ok := w.existingParts[chunkNumber]; ok {
		io.CopyN(io.Discard, reader, existing.Size)
		w.addCompletedPart(existing)
		return existing.Size, nil
	}

	var (
		response api.PartFile
		partName string
		fileName string
	)

	_, fileName = w.f.splitPathFull(w.src.Remote())

	err = w.f.pacer.Call(func() (bool, error) {

		size, err = reader.Seek(0, io.SeekEnd)
		if err != nil {

			return false, err
		}

		_, err = reader.Seek(0, io.SeekStart)
		if err != nil {
			return false, err
		}

		fs.Debugf(w.o, "Sending chunk %d length %d", chunkNumber, size)
		if w.f.opt.RandomisePart {
			u1, _ := uuid.NewV4()
			partName = hex.EncodeToString(u1.Bytes())
		} else {

			partName = fileName
			if w.totalParts > 1 {
				partName = fmt.Sprintf("%s.part.%03d", fileName, chunkNumber)
			}
		}

		opts := rest.Opts{
			Method:        "POST",
			Path:          "/api/uploads/" + w.uploadID,
			Body:          reader,
			ContentLength: &size,
			Parameters: url.Values{
				"partName":  []string{partName},
				"fileName":  []string{fileName},
				"partNo":    []string{strconv.Itoa(chunkNumber)},
				"channelId": []string{strconv.FormatInt(w.channelID, 10)},
				"encrypted": []string{strconv.FormatBool(w.encryptFile)},
			},
		}

		resp, err := w.f.srv.CallJSON(ctx, &opts, nil, &response)
		retry, err := shouldRetry(ctx, resp, err)
		if err != nil {
			fs.Debugf(w.o, "Error sending chunk %d (retry=%v): %v: %#v", chunkNumber, retry, err, err)
		}
		return retry, err

	})

	if err != nil {
		fs.Debugf(w.o, "Error sending chunk %d: %v", chunkNumber, err)
	} else {
		if response.PartId > 0 {
			w.addCompletedPart(response)
		}
		fs.Debugf(w.o, "Done sending chunk %d", chunkNumber)
	}
	return size, err

}

// add a part number and etag to the completed parts
func (w *objectChunkWriter) addCompletedPart(part api.PartFile) {
	w.partsToCommitMu.Lock()
	defer w.partsToCommitMu.Unlock()
	w.partsToCommit = append(w.partsToCommit, part)
}

func (w *objectChunkWriter) Close(ctx context.Context) error {

	if w.totalParts != int64(len(w.partsToCommit)) {
		return fmt.Errorf("uploaded failed")
	}

	base, leaf := w.f.splitPathFull(w.src.Remote())

	if base != "/" {
		err := w.f.CreateDir(ctx, base, "")
		if err != nil {
			return err
		}
	}
	opts := rest.Opts{
		Method: "POST",
		Path:   "/api/files",
	}

	sort.Slice(w.partsToCommit, func(i, j int) bool {
		return w.partsToCommit[i].PartNo < w.partsToCommit[j].PartNo
	})

	fileParts := []api.FilePart{}

	for _, part := range w.partsToCommit {
		fileParts = append(fileParts, api.FilePart{ID: part.PartId, Salt: part.Salt})
	}

	payload := api.CreateFileRequest{
		Name:      leaf,
		Type:      "file",
		Path:      base,
		MimeType:  fs.MimeType(ctx, w.src),
		Size:      w.src.Size(),
		Parts:     fileParts,
		ChannelID: w.channelID,
		Encrypted: w.encryptFile,
	}

	err := w.f.pacer.Call(func() (bool, error) {
		resp, err := w.f.srv.CallJSON(ctx, &opts, &payload, nil)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return err
	}

	opts = rest.Opts{
		Method: "DELETE",
		Path:   "/api/uploads/" + w.uploadID,
	}
	err = w.f.pacer.Call(func() (bool, error) {
		resp, err := w.f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}
	return nil
}

func (*objectChunkWriter) Abort(ctx context.Context) error {
	return nil
}

func (o *Object) prepareUpload(ctx context.Context, src fs.ObjectInfo, options []fs.OpenOption) (*uploadInfo, error) {
	base, leaf := o.fs.splitPathFull(src.Remote())

	modTime := src.ModTime(ctx).UTC().Format(timeFormat)

	uploadID := MD5(fmt.Sprintf("%s:%d:%s", path.Join(base, leaf), src.Size(), modTime))

	var uploadParts api.UploadFile
	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/uploads/" + uploadID,
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &uploadParts)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return nil, err
	}

	channelID := o.fs.opt.ChannelID

	encryptFile := o.fs.opt.EncryptFiles

	if len(uploadParts.Parts) > 0 {
		channelID = uploadParts.Parts[0].ChannelID
		encryptFile = uploadParts.Parts[0].Encrypted
	}

	return &uploadInfo{
		existingChunks: uploadParts.Parts,
		uploadID:       uploadID,
		channelID:      channelID,
		encryptFile:    encryptFile,
	}, nil
}
