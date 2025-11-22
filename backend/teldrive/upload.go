package teldrive

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/url"
	"sort"
	"strconv"
	"sync"

	"github.com/google/uuid"
	"github.com/rclone/rclone/backend/teldrive/api"
	"github.com/rclone/rclone/lib/rest"

	"github.com/rclone/rclone/fs"
)

type uploadInfo struct {
	existingChunks map[int]api.PartFile
	uploadID       string
	channelID      int64
	encryptFile    bool
	chunkSize      int64
	totalChunks    int64
	fileChunks     []api.FilePart
	fileName       string
	dir            string
}

type objectChunkWriter struct {
	size            int64
	f               *Fs
	src             fs.ObjectInfo
	partsToCommitMu sync.Mutex
	partsToCommit   []api.PartFile
	o               *Object
	uploadInfo      *uploadInfo
}

func getMD5Hash(text string) string {
	hash := md5.Sum([]byte(text))
	return hex.EncodeToString(hash[:])
}

// WriteChunk will write chunk number with reader bytes, where chunk number >= 0
func (w *objectChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (size int64, err error) {
	if chunkNumber < 0 {
		err := fmt.Errorf("invalid chunk number provided: %v", chunkNumber)
		return -1, err
	}

	chunkNumber += 1

	if existing, ok := w.uploadInfo.existingChunks[chunkNumber]; ok {
		// Discarding bytes from the reader updates the transfer stats automatically
		// because the reader passed by rclone is wrapped with accounting.
		_, _ = io.CopyN(io.Discard, reader, existing.Size)
		w.addCompletedPart(existing)
		return existing.Size, nil
	}

	var (
		response api.PartFile
		partName string
	)

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
		if w.f.opt.RandomChunkName {
			partName = getMD5Hash(uuid.New().String())
		} else {
			partName = w.uploadInfo.fileName
			if w.uploadInfo.totalChunks > 1 {
				partName = fmt.Sprintf("%s.part.%03d", w.uploadInfo.fileName, chunkNumber)
			}
		}

		opts := rest.Opts{
			Method:        "POST",
			Body:          reader,
			ContentLength: &size,
			ContentType:   "application/octet-stream",
			Parameters: url.Values{
				"partName":  []string{partName},
				"fileName":  []string{w.uploadInfo.fileName},
				"partNo":    []string{strconv.Itoa(chunkNumber)},
				"channelId": []string{strconv.FormatInt(w.uploadInfo.channelID, 10)},
				"encrypted": []string{strconv.FormatBool(w.uploadInfo.encryptFile)},
			},
		}

		if w.f.opt.UploadHost != "" {
			opts.RootURL = w.f.opt.UploadHost + "/api/uploads/" + w.uploadInfo.uploadID

		} else {
			opts.Path = "/api/uploads/" + w.uploadInfo.uploadID
		}

		resp, err := w.f.srv.CallJSON(ctx, &opts, nil, &response)

		retry, err := shouldRetry(ctx, resp, err)

		if err != nil {
			fs.Debugf(w.o, "Error sending chunk %d (retry=%v): %v: %#v", chunkNumber, retry, err, err)
		}
		if response.PartId == 0 {
			return true, fmt.Errorf("error sending chunk %d", chunkNumber)
		}

		return retry, err

	})

	if err != nil {
		return 0, fmt.Errorf("error sending chunk %d: %v", chunkNumber, err)
	}

	w.addCompletedPart(response)
	fs.Debugf(w.o, "Done sending chunk %d", chunkNumber)

	return size, err

}

// add a part number and etag to the completed parts
func (w *objectChunkWriter) addCompletedPart(part api.PartFile) {
	w.partsToCommitMu.Lock()
	defer w.partsToCommitMu.Unlock()
	w.partsToCommit = append(w.partsToCommit, part)
}

func (w *objectChunkWriter) Close(ctx context.Context) error {

	if w.uploadInfo.totalChunks != int64(len(w.partsToCommit)) {
		return fmt.Errorf("uploaded failed")
	}

	sort.Slice(w.partsToCommit, func(i, j int) bool {
		return w.partsToCommit[i].PartNo < w.partsToCommit[j].PartNo
	})

	fileChunks := []api.FilePart{}

	for _, part := range w.partsToCommit {
		fileChunks = append(fileChunks, api.FilePart{ID: part.PartId, Salt: part.Salt})
	}

	w.uploadInfo.fileChunks = fileChunks

	return w.o.createFile(ctx, w.src, w.uploadInfo)
}

func (*objectChunkWriter) Abort(ctx context.Context) error {
	return nil
}

func (o *Object) prepareUpload(ctx context.Context, src fs.ObjectInfo) (*uploadInfo, error) {

	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, src.Remote(), true)

	if err != nil {
		return nil, err
	}

	uploadID := getMD5Hash(fmt.Sprintf("%s:%s:%d:%d", directoryID, leaf, src.Size(), o.fs.userId))

	var (
		uploadParts    []api.PartFile
		existingChunks map[int]api.PartFile
	)

	opts := rest.Opts{
		Method: "GET",
		Path:   "/api/uploads/" + uploadID,
	}

	chunkSize := int64(o.fs.opt.ChunkSize)

	if chunkSize < src.Size() {
		err := o.fs.pacer.Call(func() (bool, error) {
			resp, err := o.fs.srv.CallJSON(ctx, &opts, nil, &uploadParts)
			return shouldRetry(ctx, resp, err)
		})

		if err != nil {
			return nil, err
		}
		existingChunks = make(map[int]api.PartFile, len(uploadParts))
		for _, part := range uploadParts {
			existingChunks[part.PartNo] = part
		}

	}

	totalChunks := src.Size() / chunkSize

	if src.Size()%chunkSize != 0 {
		totalChunks++
	}

	channelID := o.fs.opt.ChannelID

	encryptFile := o.fs.opt.EncryptFiles

	if len(uploadParts) > 0 {
		channelID = uploadParts[0].ChannelID
		encryptFile = uploadParts[0].Encrypted
	}

	return &uploadInfo{
		existingChunks: existingChunks,
		uploadID:       uploadID,
		channelID:      channelID,
		encryptFile:    encryptFile,
		chunkSize:      chunkSize,
		totalChunks:    totalChunks,
		fileName:       leaf,
		dir:            directoryID,
	}, nil
}

func (o *Object) uploadMultipart(ctx context.Context, in io.Reader, src fs.ObjectInfo) (*uploadInfo, error) {

	size := src.Size()

	if size < 0 {
		return nil, errors.New("unknown-sized upload not supported")
	}

	uploadInfo, err := o.prepareUpload(ctx, src)

	if err != nil {
		return nil, err
	}

	if size > 0 {

		var (
			partsToCommit []api.PartFile
			uploadedSize  int64
		)

		totalChunks := int(uploadInfo.totalChunks)

		for chunkNo := 1; chunkNo <= totalChunks; chunkNo++ {
			if existing, ok := uploadInfo.existingChunks[chunkNo]; ok {
				io.CopyN(io.Discard, in, existing.Size)
				partsToCommit = append(partsToCommit, existing)
				uploadedSize += existing.Size
				continue
			}

			n := uploadInfo.chunkSize

			if chunkNo == totalChunks {
				n = src.Size() - uploadedSize
			}

			chunkName := uploadInfo.fileName

			if o.fs.opt.RandomChunkName {
				chunkName = getMD5Hash(uuid.New().String())
			} else if totalChunks > 1 {
				chunkName = fmt.Sprintf("%s.part.%03d", chunkName, chunkNo)
			}

			partReader := io.LimitReader(in, n)

			opts := rest.Opts{
				Method:        "POST",
				Body:          partReader,
				ContentLength: &n,
				ContentType:   "application/octet-stream",
				Parameters: url.Values{
					"partName":  []string{chunkName},
					"fileName":  []string{uploadInfo.fileName},
					"partNo":    []string{strconv.Itoa(chunkNo)},
					"channelId": []string{strconv.FormatInt(uploadInfo.channelID, 10)},
					"encrypted": []string{strconv.FormatBool(uploadInfo.encryptFile)},
				},
			}

			if o.fs.opt.UploadHost != "" {
				opts.RootURL = o.fs.opt.UploadHost + "/api/uploads/" + uploadInfo.uploadID

			} else {
				opts.Path = "/api/uploads/" + uploadInfo.uploadID
			}

			var partInfo api.PartFile

			_, err := o.fs.srv.CallJSON(ctx, &opts, nil, &partInfo)

			if err != nil {
				return nil, err
			}

			uploadedSize += n

			partsToCommit = append(partsToCommit, partInfo)
		}

		sort.Slice(partsToCommit, func(i, j int) bool {
			return partsToCommit[i].PartNo < partsToCommit[j].PartNo
		})

		fileChunks := []api.FilePart{}

		for _, part := range partsToCommit {
			fileChunks = append(fileChunks, api.FilePart{ID: part.PartId, Salt: part.Salt})
		}

		uploadInfo.fileChunks = fileChunks
	}

	return uploadInfo, nil

}

func (o *Object) createFile(ctx context.Context, src fs.ObjectInfo, uploadInfo *uploadInfo) error {

	opts := rest.Opts{
		Method:     "POST",
		Path:       "/api/files",
		NoResponse: true,
	}

	payload := api.CreateFileRequest{
		Name:      uploadInfo.fileName,
		Type:      "file",
		ParentId:  uploadInfo.dir,
		MimeType:  fs.MimeType(ctx, src),
		Size:      src.Size(),
		Parts:     uploadInfo.fileChunks,
		ChannelID: uploadInfo.channelID,
		Encrypted: uploadInfo.encryptFile,
		ModTime:   src.ModTime(ctx).UTC(),
	}

	err := o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallJSON(ctx, &opts, &payload, nil)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		return err
	}
	if src.Size() > 0 {
		opts = rest.Opts{
			Method:     "DELETE",
			Path:       "/api/uploads/" + uploadInfo.uploadID,
			NoResponse: true,
		}

		err = o.fs.pacer.Call(func() (bool, error) {
			resp, err := o.fs.srv.Call(ctx, &opts)
			return shouldRetry(ctx, resp, err)
		})
		if err != nil {
			return err
		}
	}
	return nil
}