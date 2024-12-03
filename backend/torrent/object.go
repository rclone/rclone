package torrent

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
)

// Object implements a torrent file object
type Object struct {
	fs          *Fs
	entry       *TorrentEntry
	virtualPath string // Path as seen by rclone
	torrentPath string // Path within the torrent
	size        int64
	modTime     time.Time
}

// Standard Object interface implementations
func (o *Object) Remote() string                        { return o.virtualPath }
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }
func (o *Object) Size() int64                           { return o.size }
func (o *Object) Fs() fs.Info                           { return o.fs }
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}
func (o *Object) Storable() bool { return true }
func (o *Object) String() string { return o.virtualPath }

// Read-only implementations
func (o *Object) SetModTime(ctx context.Context, t time.Time) error { return fs.ErrorPermissionDenied }
func (o *Object) Remove(ctx context.Context) error                  { return fs.ErrorPermissionDenied }
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return fs.ErrorPermissionDenied
}

// torrentReader wraps torrent.Reader with enhanced piece management
type torrentReader struct {
	ctx        context.Context
	reader     torrent.Reader
	object     *Object
	readAhead  int
	pieceSize  int64
	lastPiece  int64
	bytesRead  int64
	lastUpdate time.Time
	deadline   time.Time // Add deadline tracking
}

// Open implements fs.Object
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Update access time and enable downloading
	o.entry.accessed = time.Now()
	o.entry.torrent.AllowDataDownload()

	// Find target file in torrent
	var targetFile *torrent.File
	for _, file := range o.entry.torrent.Files() {
		if file.Path() == o.torrentPath {
			targetFile = file
			break
		}
	}
	if targetFile == nil {
		return nil, fmt.Errorf("file not found in torrent: %s", o.torrentPath)
	}

	// Get torrent info
	info := o.entry.torrent.Info()
	if info == nil {
		return nil, fmt.Errorf("torrent info not available")
	}

	// Create reader
	reader := &torrentReader{
		ctx:        ctx,
		reader:     targetFile.NewReader(),
		object:     o,
		readAhead:  o.fs.opt.PieceReadAhead,
		pieceSize:  int64(info.PieceLength),
		lastUpdate: time.Now(),
	}

	// Handle initial seek if requested
	for _, option := range options {
		if opt, ok := option.(*fs.SeekOption); ok {
			_, err := reader.reader.Seek(opt.Offset, io.SeekStart)
			if err != nil {
				reader.Close()
				return nil, fmt.Errorf("seek error: %w", err)
			}
			break
		}
	}

	// Initialize piece priorities
	reader.updatePriorities()

	return reader, nil
}

// Read implements io.Reader
func (r *torrentReader) Read(p []byte) (n int, err error) {
	// Get current position
	offset, err := r.reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("failed to get position: %w", err)
	}

	// Calculate current piece
	currentPiece := offset / r.pieceSize

	// Update priorities if we've moved to a new piece
	if currentPiece != r.lastPiece {
		r.updatePriorities()
		r.lastPiece = currentPiece
	}

	// Wait for pieces to be available
	startPiece := offset / r.pieceSize
	endPiece := (offset+int64(len(p)))/r.pieceSize + 1

	// Wait for pieces with timeout
	timeout := time.NewTimer(30 * time.Second)
	defer timeout.Stop()

	for {
		hasAllPieces := true
		for i := startPiece; i <= endPiece; i++ {
			if i >= int64(r.object.entry.torrent.Info().NumPieces()) {
				break
			}
			if !r.object.entry.torrent.Piece(int(i)).State().Complete {
				hasAllPieces = false
				break
			}
		}

		if hasAllPieces {
			break
		}

		select {
		case <-r.ctx.Done():
			return 0, r.ctx.Err()
		case <-timeout.C:
			return 0, fmt.Errorf("timeout waiting for pieces")
		case <-time.After(100 * time.Millisecond):
			continue
		}
	}

	// Perform actual read
	n, err = r.reader.Read(p)
	if err == nil {
		r.bytesRead += int64(n)
		r.logProgress()
	}
	return n, err
}

// updatePriorities sets piece priorities based on current read position
func (r *torrentReader) updatePriorities() {
	info := r.object.entry.torrent.Info()
	if info == nil {
		return
	}

	offset, err := r.reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return
	}

	// Calculate needed pieces range
	startPiece := offset / r.pieceSize
	endPiece := (offset+r.bytesRead)/r.pieceSize + 1

	// Check if we have all needed pieces
	hasAllPieces := true
	for i := startPiece; i <= endPiece; i++ {
		if i >= int64(info.NumPieces()) {
			break
		}
		if !r.object.entry.torrent.Piece(int(i)).State().Complete {
			hasAllPieces = false
			break
		}
	}

	if !hasAllPieces {
		// Reset all piece priorities first
		for i := 0; i < info.NumPieces(); i++ {
			piece := r.object.entry.torrent.Piece(i)
			if !piece.State().Complete {
				piece.SetPriority(torrent.PiecePriorityNone)
			}
		}

		// Set high priority for needed pieces and next 4 pieces
		deadline := time.Now().Add(10 * time.Second)
		for i := endPiece; i < min(endPiece+4, int64(info.NumPieces())); i++ {
			piece := r.object.entry.torrent.Piece(int(i))
			if !piece.State().Complete {
				piece.SetPriority(torrent.PiecePriorityHigh) // Equivalent to priority 7
				// Note: anacrolix/torrent doesn't have direct deadline support,
				// so we implement a similar behavior in Read
			}
			deadline = deadline.Add(-time.Second) // Decreasing deadlines
		}
		r.deadline = deadline
	}
}

// logProgress logs download progress periodically
func (r *torrentReader) logProgress() {
	now := time.Now()
	if now.Sub(r.lastUpdate) < 2*time.Second {
		return
	}

	stats := r.object.entry.torrent.Stats()
	fs.Debugf(r.object, "Progress: read %s, peers=%d/%d",
		fs.SizeSuffix(r.bytesRead),
		stats.ActivePeers,
		stats.TotalPeers)

	r.lastUpdate = now
}

// Close implements io.Closer
func (r *torrentReader) Close() error {
	fs.Debugf(r.object, "Closing reader after reading %s", fs.SizeSuffix(r.bytesRead))
	return r.reader.Close()
}

type readResult struct {
	n   int
	err error
}

// Helper function
func min(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

// Check interface implementation
var _ fs.Object = (*Object)(nil)
