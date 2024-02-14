// Package serve deals with serving objects over HTTP
package serve

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
)

// Object serves an fs.Object via HEAD or GET
func Object(w http.ResponseWriter, r *http.Request, o fs.Object) {
	if r.Method != "HEAD" && r.Method != "GET" {
		http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
		return
	}

	// Show that we accept ranges
	w.Header().Set("Accept-Ranges", "bytes")

	// Set content length since we know how long the object is
	if o.Size() >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(o.Size(), 10))
	}

	// Set content type
	mimeType := fs.MimeType(r.Context(), o)
	if mimeType == "application/octet-stream" && path.Ext(o.Remote()) == "" {
		// Leave header blank so http server guesses
	} else {
		w.Header().Set("Content-Type", mimeType)
	}

	// Set last modified
	modTime := o.ModTime(r.Context())
	w.Header().Set("Last-Modified", modTime.UTC().Format(http.TimeFormat))

	if r.Method == "HEAD" {
		return
	}

	// Decode Range request if present
	code := http.StatusOK
	size := o.Size()
	var options []fs.OpenOption
	if rangeRequest := r.Header.Get("Range"); rangeRequest != "" {
		//fs.Debugf(nil, "Range: request %q", rangeRequest)
		option, err := fs.ParseRangeOption(rangeRequest)
		if err != nil {
			fs.Debugf(o, "Get request parse range request error: %v", err)
			http.Error(w, http.StatusText(http.StatusBadRequest), http.StatusBadRequest)
			return
		}
		options = append(options, option)
		offset, limit := option.Decode(o.Size())
		end := o.Size() // exclusive
		if limit >= 0 {
			end = offset + limit
		}
		if end > o.Size() {
			end = o.Size()
		}
		size = end - offset
		// fs.Debugf(nil, "Range: offset=%d, limit=%d, end=%d, size=%d (object size %d)", offset, limit, end, size, o.Size())
		// Content-Range: bytes 0-1023/146515
		w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", offset, end-1, o.Size()))
		// fs.Debugf(nil, "Range: Content-Range: %q", w.Header().Get("Content-Range"))
		code = http.StatusPartialContent
	}
	w.Header().Set("Content-Length", strconv.FormatInt(size, 10))

	file, err := o.Open(r.Context(), options...)
	if err != nil {
		fs.Debugf(o, "Get request open error: %v", err)
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
		return
	}
	tr := accounting.Stats(r.Context()).NewTransfer(o, nil)
	defer func() {
		tr.Done(r.Context(), err)
	}()
	in := tr.Account(r.Context(), file) // account the transfer (no buffering)

	w.WriteHeader(code)

	n, err := io.Copy(w, in)
	if err != nil {
		fs.Errorf(o, "Didn't finish writing GET request (wrote %d/%d bytes): %v", n, size, err)
		return
	}
}
