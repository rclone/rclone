// http_handler.go - Custom HTTP handler for streaming
package feb_box

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/rclone/rclone/fs"
)

// StreamHandler handles HTTP requests for streaming
type StreamHandler struct {
    fs *Fs
}

func (h *StreamHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // Extract file path
    path := strings.TrimPrefix(r.URL.Path, "/")
    if path == "" {
        http.Error(w, "File not specified", http.StatusBadRequest)
        return
    }

    ctx := r.Context()
    
    // Get the object
    obj, err := h.fs.NewObject(ctx, path)
    if err != nil {
        http.Error(w, "File not found", http.StatusNotFound)
        return
    }

    // Set headers for streaming
    w.Header().Set("Content-Type", obj.(*Object).mimeType)
    w.Header().Set("Accept-Ranges", "bytes")
    w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
    w.Header().Set("Pragma", "no-cache")
    w.Header().Set("Expires", "0")
    
    // Handle range requests
    rangeHeader := r.Header.Get("Range")
    if rangeHeader != "" {
        w.Header().Set("Content-Range", fmt.Sprintf("bytes %s/%d", strings.TrimPrefix(rangeHeader, "bytes="), obj.Size()))
        w.WriteHeader(http.StatusPartialContent)
    }

    // Open the file
    var rc io.ReadCloser
    if rangeHeader != "" {
        // Parse range
        var start, end int64
        fmt.Sscanf(rangeHeader, "bytes=%d-%d", &start, &end)
        rc, err = obj.Open(ctx, &fs.RangeOption{Start: start, End: end})
    } else {
        rc, err = obj.Open(ctx)
    }
    
    if err != nil {
        http.Error(w, "Failed to open file", http.StatusInternalServerError)
        return
    }
    defer rc.Close()

    // Stream the file
    io.Copy(w, rc)
}