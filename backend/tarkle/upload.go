package tarkle

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/tarkle/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

const (
	maxPartURLBatch    = 100             // POST /uploads/{id}/parts takes up to 100
	maxResendRounds    = 3               // times Close re-sends parts the server reports missing
	maxFinalizeRetries = 5               // times Close asks again after a 409 "finalizing"
	finalizeRetryDelay = 2 * time.Second // pause between those asks
)

// partPlan describes how a file is cut into parts: all parts are
// partSize bytes except the last one.
type partPlan struct {
	size     int64
	partSize int64
	count    int
}

// newPartPlan checks the server's part size and count against the size
// of the file. A single upload is one part of the whole file.
func newPartPlan(size, partSize int64, count int, single bool) (partPlan, error) {
	if single {
		return partPlan{size: size, partSize: size, count: 1}, nil
	}
	if partSize <= 0 || count <= 0 {
		return partPlan{}, fmt.Errorf("server returned part_size %d and part_count %d", partSize, count)
	}
	want := int((size + partSize - 1) / partSize)
	if want != count {
		return partPlan{}, fmt.Errorf("server says %d parts of %d bytes but a file of %d bytes needs %d", count, partSize, size, want)
	}
	return partPlan{size: size, partSize: partSize, count: count}, nil
}

// bounds returns the offset and length of part number n (1 based)
func (p partPlan) bounds(n int) (off, length int64, ok bool) {
	if n < 1 || n > p.count {
		return 0, 0, false
	}
	off = int64(n-1) * p.partSize
	length = min(p.partSize, p.size-off)
	return off, length, true
}

// missingParts works out which parts a 409 incomplete answer says are
// missing. The details may give the list of parts that are missing
// (missing_parts), or the list of parts received or needed; counts
// alone can't say which parts to resend and give ok == false.
func missingParts(d api.IncompleteDetails, count int) (missing []int, ok bool) {
	if len(d.MissingParts) > 0 {
		out := slices.Clone(d.MissingParts)
		slices.Sort(out)
		return out, true
	}
	var received []int
	if json.Unmarshal(d.PartsReceived, &received) != nil {
		return nil, false
	}
	have := make(map[int]bool, len(received))
	for _, n := range received {
		have[n] = true
	}
	for n := 1; n <= count; n++ {
		if !have[n] {
			missing = append(missing, n)
		}
	}
	return missing, true
}

// chunkWriter is an fs.ChunkWriter over a Tarkle upload
type chunkWriter struct {
	f           *Fs
	src         fs.ObjectInfo
	leaf        string // name sent to the server (encoded)
	dirID       string
	plan        partPlan
	id          string // upload id, "" for an empty file
	multipartID string // only for new versions above 100 MiB
	renamed     bool   // the server said the name would be changed
	modTime     time.Time
	sha256      string // of the source if it was cheap to get, "" if not

	mu    sync.Mutex
	urls  map[int]string
	entry api.Item
	done  bool // nothing to upload (empty file or identical content)
}

// startUpload does POST /uploads, answering the app's duplicate
// questions: when a same name file makes the server ask (409 conflict),
// the file is being overwritten, so ask for a new version, or for a
// separate file (renamed back after) where versions are not possible.
func (f *Fs) startUpload(ctx context.Context, leaf, dirID string, size int64, mime string, modTime time.Time) (*api.UploadStarted, error) {
	req := api.StartUpload{Name: f.opt.Enc.FromStandardName(leaf), Size: size, ContentType: mime}
	if size == 0 && !modTime.IsZero() {
		req.ModifiedAt = formatTime(modTime) // an empty file has no complete step
	}
	if dirID != "" {
		req.FolderID = &dirID
	}
	for attempt := 0; ; attempt++ {
		var res api.UploadStarted
		opts := rest.Opts{Method: "POST", Path: "/uploads"}
		err := f.call(ctx, &opts, &req, &res)
		if err == nil {
			return &res, nil
		}
		e := apiError(err)
		if e == nil || e.Status != http.StatusConflict || attempt > 0 {
			return nil, fmt.Errorf("failed to start upload: %w", err)
		}
		switch e.Code() {
		case "conflict":
			req.OnConflict = "version"
			if size == 0 || !choiceOffered(e, "version") {
				req.OnConflict = "separate"
			}
		case "empty_version_not_supported", "version_upload_not_available":
			req.OnConflict = "separate"
		default:
			return nil, fmt.Errorf("failed to start upload: %w", err)
		}
	}
}

// choiceOffered says if a 409 conflict lists want among its choices
// (when it lists none, any choice is assumed to be allowed)
func choiceOffered(e *api.Error, want string) bool {
	var d struct {
		Choices []string `json:"choices"`
	}
	if json.Unmarshal(e.Wrapper.Details, &d) != nil || len(d.Choices) == 0 {
		return true
	}
	return slices.Contains(d.Choices, want)
}

// OpenChunkWriter returns the chunk size and a ChunkWriter
func (f *Fs) OpenChunkWriter(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (info fs.ChunkWriterInfo, writer fs.ChunkWriter, err error) {
	size := src.Size()
	if size < 0 {
		return info, nil, errors.New("tarkle needs the size of a file before it is uploaded")
	}
	leaf, dirID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return info, nil, err
	}
	modTime := src.ModTime(ctx)
	res, err := f.startUpload(ctx, leaf, dirID, size, fs.MimeType(ctx, src), modTime)
	if err != nil {
		return info, nil, err
	}
	orig := src
	if or, ok := src.(*fs.OverrideRemote); ok {
		orig = or.ObjectInfo // so the source can be read again by range
	}
	w := &chunkWriter{
		f:       f,
		src:     orig,
		leaf:    f.opt.Enc.FromStandardName(leaf),
		dirID:   dirID,
		urls:    map[int]string{},
		renamed: res.WillBeRenamed,
		modTime: modTime,
	}
	// send the SHA-256 on complete when the source can give it
	if sum, err := orig.Hash(ctx, hash.SHA256); err == nil && sum != "" {
		w.sha256 = sum
	}
	info = fs.ChunkWriterInfo{Concurrency: f.opt.UploadConcurrency}
	switch res.Action {
	case "created": // empty file: no upload step
		if res.File == nil {
			return info, nil, errors.New("server created an empty file but sent no entry")
		}
		w.entry, w.done = *res.File, true
		info.ChunkSize = 1
	case "skip": // identical content is already there
		existing, _, err := f.readMetaDataForPath(ctx, remote)
		if err != nil {
			return info, nil, fmt.Errorf("upload skipped as identical but the file can't be read: %w", err)
		}
		w.entry, w.done = *existing, true
		info.ChunkSize = max(1, size)
	case "upload":
		w.plan, err = newPartPlan(size, res.PartSize, res.PartCount, res.Mode == "single")
		if err != nil {
			return info, nil, err
		}
		w.id, w.multipartID = res.ID, res.MultipartID
		for _, u := range res.Uploads {
			w.urls[u.PartNumber] = u.URL
		}
		info.ChunkSize = w.plan.partSize
		info.LeavePartsOnError = false
	default:
		return info, nil, fmt.Errorf("unexpected upload action %q", res.Action)
	}
	return info, w, nil
}

// partURL returns the presigned URL for part n, asking the server for
// a batch of fresh ones when it has none (or when refresh is set).
func (w *chunkWriter) partURL(ctx context.Context, n int, refresh bool) (string, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if u, ok := w.urls[n]; ok && !refresh {
		return u, nil
	}
	count := 1
	if !refresh {
		count = min(maxPartURLBatch, w.plan.count-n+1)
	}
	var res api.PartsResponse
	opts := rest.Opts{Method: "POST", Path: "/uploads/" + w.id + "/parts"}
	err := w.f.call(ctx, &opts, &api.PartsRequest{FromPart: n, Count: count, MultipartID: w.multipartID}, &res)
	if err != nil {
		return "", fmt.Errorf("couldn't get upload URLs for part %d: %w", n, err)
	}
	for _, u := range res.Uploads {
		w.urls[u.PartNumber] = u.URL
	}
	u, ok := w.urls[n]
	if !ok {
		return "", fmt.Errorf("server sent no URL for part %d", n)
	}
	return u, nil
}

// putPart sends the bytes of one part to its presigned URL. Only this
// part is retried: with the same URL after a network error or a 5xx,
// with a fresh URL after a 401 or 403 (the URL expired).
func (w *chunkWriter) putPart(ctx context.Context, n int, body io.ReadSeeker, length int64) error {
	refresh := false
	return w.f.storePacer.Call(func() (bool, error) {
		u, err := w.partURL(ctx, n, refresh)
		if err != nil {
			return false, err
		}
		if _, err = body.Seek(0, io.SeekStart); err != nil {
			return false, err
		}
		opts := rest.Opts{Method: "PUT", RootURL: u, Body: body, ContentLength: &length, NoResponse: true}
		resp, err := w.f.store.Call(ctx, &opts)
		if resp != nil && (resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusUnauthorized) {
			refresh = true
			return true, fmt.Errorf("part %d: URL rejected (%s), asking for a new one: %w", n, resp.Status, err)
		}
		if err != nil {
			err = fmt.Errorf("part %d: %w", n, err)
		}
		return shouldRetry(ctx, resp, err)
	})
}

// WriteChunk writes chunk number (0 based) with the bytes in reader
func (w *chunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	length, err := reader.Seek(0, io.SeekEnd)
	if err != nil {
		return 0, fmt.Errorf("failed to seek chunk: %w", err)
	}
	if w.done {
		return length, nil // nothing to send: the file exists already
	}
	n := chunkNumber + 1
	_, want, ok := w.plan.bounds(n)
	if !ok {
		return 0, fmt.Errorf("chunk %d is beyond the %d parts of this upload", n, w.plan.count)
	}
	if length != want {
		return 0, fmt.Errorf("part %d has %d bytes, the server expects %d", n, length, want)
	}
	if err = w.putPart(ctx, n, reader, length); err != nil {
		return 0, err
	}
	// A single-part upload is the whole file in one WriteChunk call: hashing the bytes that were
	// actually sent (not just trusting whatever orig.Hash() claimed before any byte moved) lets the
	// server verify what truly went over the wire, and gives rclone's own post-upload Object.Hash()
	// check something real to compare against. A wrapping Fs that substitutes different bytes than
	// its ObjectInfo claims (a test double, or any real middleware bug) is otherwise invisible: the
	// claimed hash still matches on both sides even though the stored content doesn't. Multi-part
	// uploads keep the opportunistic claim: hashing chunks that may arrive out of order under
	// concurrency would mean buffering the whole file just to verify it, defeating the point of
	// streaming large uploads.
	if w.plan.count == 1 {
		if _, err := reader.Seek(0, io.SeekStart); err != nil {
			return 0, fmt.Errorf("failed to seek chunk for hashing: %w", err)
		}
		sums, err := hash.StreamTypes(reader, hash.NewHashSet(hash.SHA256))
		if err == nil {
			w.sha256 = sums[hash.SHA256]
		}
	}
	return length, nil
}

// complete asks the server to finish the upload
func (w *chunkWriter) complete(ctx context.Context) (api.Item, error) {
	var item api.Item
	opts := rest.Opts{Method: "POST", Path: "/uploads/" + w.id + "/complete"}
	req := api.CompleteRequest{MultipartID: w.multipartID, SHA256: w.sha256}
	if !w.modTime.IsZero() {
		req.ModifiedAt = formatTime(w.modTime)
	}
	err := w.f.call(ctx, &opts, &req, &item)
	return item, err
}

// resend sends the parts again by reading them from the source
func (w *chunkWriter) resend(ctx context.Context, parts []int) error {
	obj, ok := w.src.(fs.Object)
	if !ok {
		return errors.New("can't re-read the source to resend missing parts")
	}
	for _, n := range parts {
		off, length, ok := w.plan.bounds(n)
		if !ok {
			return fmt.Errorf("server asked for part %d of %d", n, w.plan.count)
		}
		in, err := obj.Open(ctx, &fs.RangeOption{Start: off, End: off + length - 1})
		if err != nil {
			return fmt.Errorf("re-reading part %d: %w", n, err)
		}
		data, err := io.ReadAll(in)
		_ = in.Close()
		if err != nil {
			return fmt.Errorf("re-reading part %d: %w", n, err)
		}
		fs.Infof(w.src, "resending part %d", n)
		if err = w.putPart(ctx, n, newBytesReader(data), int64(len(data))); err != nil {
			return err
		}
	}
	return nil
}

// Close completes the upload. If the server reports parts missing it
// reads just those from the source again and tries once more.
func (w *chunkWriter) Close(ctx context.Context) (err error) {
	if w.done {
		return w.fixName(ctx) // an empty file may have been renamed too
	}
	finalizing := 0
	for round := 0; ; round++ {
		w.entry, err = w.complete(ctx)
		e := apiError(err)
		if e != nil && e.Code() == "integrity_mismatch" {
			// the server read the stored bytes and they differ from the source's SHA-256: it binned the file
			return fserrors.RetryError(fmt.Errorf("upload corrupted in transit, starting again: %w", err))
		}
		if e != nil && e.Code() == "finalizing" && finalizing < maxFinalizeRetries {
			// another complete call for this same upload is already in flight server side (our own retry
			// racing an earlier attempt that did land, or two clients finishing the same upload at once):
			// the server said to ask again shortly, not that anything is wrong, so just do that. Kept on
			// its own counter so it never eats into the part-resend budget below.
			finalizing++
			round--
			select {
			case <-time.After(finalizeRetryDelay):
			case <-ctx.Done():
				return ctx.Err()
			}
			continue
		}
		if err == nil || e == nil || e.Code() != "incomplete" || round >= maxResendRounds {
			break
		}
		var d api.IncompleteDetails
		_ = json.Unmarshal(e.Wrapper.Details, &d)
		missing, ok := missingParts(d, w.plan.count)
		if !ok || len(missing) == 0 {
			break
		}
		fs.Debugf(w.src, "server is missing parts %v", missing)
		if err = w.resend(ctx, missing); err != nil {
			return err
		}
	}
	if err != nil {
		return fmt.Errorf("failed to complete upload: %w", err)
	}
	return w.fixName(ctx)
}

// fixName puts the file under its own name if the server renamed it
// to avoid a duplicate ("name (2)"): the old file is replaced.
func (w *chunkWriter) fixName(ctx context.Context) error {
	if w.entry.Name == w.leaf {
		return nil
	}
	if w.entry.Name != "" && !w.renamed && w.entry.ID != "" {
		// renamed although the server did not announce it: still fix
		fs.Debugf(w.src, "server saved %q instead of %q", w.entry.Name, w.leaf)
	}
	return w.f.replaceExisting(ctx, w.dirID, w.leaf, &w.entry)
}

// replaceExisting removes the file called leaf (encoded) in dirID that
// is not item, then renames item to leaf
func (f *Fs) replaceExisting(ctx context.Context, dirID, leaf string, item *api.Item) error {
	var old string
	_, err := f.listAll(ctx, dirID, api.ItemTypeFile, false, func(i *api.Item) bool {
		if f.opt.Enc.FromStandardName(i.Name) == leaf && i.ID != item.ID {
			old = i.ID
			return true
		}
		return false
	})
	if err != nil {
		return err
	}
	if old != "" {
		if err = f.deleteObject(ctx, old); err != nil {
			return fmt.Errorf("couldn't remove the file being replaced: %w", err)
		}
	}
	renamed, err := f.patch(ctx, item.ID, api.Patch{Name: &leaf})
	if err != nil {
		return err
	}
	*item = *renamed
	return nil
}

// Abort the upload (no effect if it was never started on the server)
func (w *chunkWriter) Abort(ctx context.Context) error {
	if w.id == "" || w.done {
		return nil
	}
	opts := rest.Opts{Method: "DELETE", Path: "/uploads/" + w.id, NoResponse: true, Parameters: map[string][]string{}}
	if w.multipartID != "" {
		opts.Parameters.Set("multipart_id", w.multipartID)
	}
	if err := w.f.call(ctx, &opts, nil, nil); err != nil {
		return fmt.Errorf("failed to abort upload: %w", err)
	}
	return nil
}

// newBytesReader is a ReadSeeker over data
func newBytesReader(data []byte) io.ReadSeeker { return bytes.NewReader(data) }
