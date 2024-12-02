package torrent

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/types"
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

// pieceConfig encapsulates piece management configuration
type pieceConfig struct {
	piece        int64 // Current piece number
	length       int64 // Piece length
	totalPieces  int   // Total number of pieces
	readAhead    int   // Number of pieces to read ahead
	offset       int64 // Current offset
	priorityHigh int   // Number of high priority pieces
}

// Open implements fs.Object
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Update access time and enable downloading
	o.entry.accessed = time.Now()
	o.entry.torrent.AllowDataDownload()

	// Find target file in torrent
	targetFile := o.findTargetFile()
	if targetFile == nil {
		return nil, fmt.Errorf("file not found in torrent (looking for %q)", o.torrentPath)
	}

	// Get torrent info and validate
	info := o.entry.torrent.Info()
	if info == nil {
		return nil, fmt.Errorf("torrent info not available")
	}

	// Calculate offset from options
	offset := o.getOffsetFromOptions(options)

	// Create reader
	reader := targetFile.NewReader()

	// Configure read-ahead based on file type
	readAhead := o.determineReadAhead()
	fs.Debugf(o, "Using read-ahead value: %d pieces", readAhead)

	// Initialize enhanced reader
	enhancedRdr := &enhancedReader{
		ctx:           ctx,
		reader:        reader,
		object:        o,
		readAhead:     readAhead,
		pieceLength:   int64(info.PieceLength),
		lastLogUpdate: time.Now(),
	}

	// Handle initial positioning
	if err := enhancedRdr.initializePosition(offset, info); err != nil {
		reader.Close()
		return nil, err
	}

	return enhancedRdr, nil
}

// findTargetFile locates the correct file within the torrent
func (o *Object) findTargetFile() *torrent.File {
	for _, file := range o.entry.torrent.Files() {
		if file.Path() == o.torrentPath {
			return file
		}
	}
	return nil
}

// getOffsetFromOptions extracts seek offset from options
func (o *Object) getOffsetFromOptions(options []fs.OpenOption) int64 {
	for _, option := range options {
		if opt, ok := option.(*fs.SeekOption); ok {
			return opt.Offset
		}
	}
	return 0
}

// determineReadAhead calculates appropriate read-ahead value
func (o *Object) determineReadAhead() int {
	baseReadAhead := o.fs.opt.PieceReadAhead
	if isMediaFile(o.virtualPath) {
		info := o.entry.torrent.Info()
		if info != nil {
			return calculateMediaReadAhead(o.size, int64(info.PieceLength), baseReadAhead)
		}
	}
	return baseReadAhead
}

// enhancedReader provides advanced reading capabilities
type enhancedReader struct {
	ctx           context.Context
	reader        torrent.Reader
	object        *Object
	bytesRead     int64
	lastLogUpdate time.Time
	readAhead     int   // Current read-ahead setting
	pieceLength   int64 // Cached piece length
}

// Read implements io.Reader with enhanced piece handling
func (r *enhancedReader) Read(p []byte) (n int, err error) {
	info := r.object.entry.torrent.Info()
	if info == nil {
		return 0, fmt.Errorf("torrent info not available")
	}

	currentOffset, err := r.reader.Seek(0, io.SeekCurrent)
	if err != nil {
		return 0, fmt.Errorf("failed to get current position: %w", err)
	}

	// Calculate piece information
	startPiece := currentOffset / r.pieceLength
	endPiece := (currentOffset + int64(len(p)) - 1) / r.pieceLength

	fs.Debugf(r.object, "Read request at offset %d (piece %d), length %d (ending at piece %d)",
		currentOffset, startPiece, len(p), endPiece)

	// Ensure proper piece priorities
	r.ensurePiecePriorities(startPiece, endPiece)

	// Perform read with timeout handling
	readChan := make(chan readResult, 1)
	doneChan := make(chan struct{})

	go func() {
		defer close(readChan)
		n, err := r.reader.Read(p)
		select {
		case readChan <- readResult{n, err}:
		case <-doneChan:
		}
	}()

	select {
	case result, ok := <-readChan:
		if !ok {
			return 0, fmt.Errorf("read operation cancelled")
		}
		return r.processReadResult(result, currentOffset, startPiece)

	case <-time.After(10 * time.Second):
		close(doneChan)
		r.logTimeoutState(currentOffset, startPiece, endPiece)
		return 0, fmt.Errorf("read timeout at offset %d (pieces %d-%d)",
			currentOffset, startPiece, endPiece)
	}
}

// processReadResult handles the result of a read operation
func (r *enhancedReader) processReadResult(result readResult, offset, startPiece int64) (int, error) {
	if result.err == nil {
		atomic.AddInt64(&r.bytesRead, int64(result.n))
		r.logProgress(result.n)

		endOffset := offset + int64(result.n)
		actualEndPiece := (endOffset - 1) / r.pieceLength
		fs.Debugf(r.object, "Completed read of %d bytes, offset %d-%d (pieces %d-%d)",
			result.n, offset, endOffset-1, startPiece, actualEndPiece)
	} else if result.err != io.EOF {
		fs.Debugf(r.object, "Read error at offset %d (piece %d): %v",
			offset, startPiece, result.err)
	}
	return result.n, result.err
}

// initializePosition sets up initial piece priorities and position
func (r *enhancedReader) initializePosition(offset int64, info *metainfo.Info) error {
	if offset == 0 {
		return r.initializeSequentialRead(info)
	}
	return r.seekAndPrioritize(offset)
}

// initializeSequentialRead sets up sequential read from the start
func (r *enhancedReader) initializeSequentialRead(info *metainfo.Info) error {
	// Clear existing priorities
	for i := 0; i < info.NumPieces(); i++ {
		piece := r.object.entry.torrent.Piece(i)
		if !piece.State().Complete {
			piece.SetPriority(types.PiecePriorityNone)
		}
	}

	// Set initial piece priorities
	startPiece := int64(0)
	numPieces := int64(info.NumPieces())
	endPiece := startPiece + int64(r.readAhead)
	if endPiece >= numPieces {
		endPiece = numPieces - 1
	}

	fs.Debugf(r.object, "Setting initial sequential read priorities for pieces %d-%d",
		startPiece, endPiece)

	for i := startPiece; i <= endPiece; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		if !piece.State().Complete {
			priority := types.PiecePriorityNormal
			if i <= startPiece+4 {
				priority = types.PiecePriorityReadahead
			}
			piece.SetPriority(priority)
		}
	}

	return nil
}

// seekAndPrioritize handles seeking and updating piece priorities
func (r *enhancedReader) seekAndPrioritize(offset int64) error {
	info := r.object.entry.torrent.Info()
	if info == nil {
		return fmt.Errorf("torrent info not available during seek")
	}

	pieceSize := int64(info.PieceLength)
	startPiece := offset / pieceSize
	numPieces := int64(info.NumPieces())

	fs.Debugf(r.object, "Starting seek operation to offset %d (piece %d/%d)",
		offset, startPiece, numPieces)

	// Clear and reset priorities
	r.clearPiecePriorities(info)
	if err := r.setPriorityWindow(startPiece, numPieces); err != nil {
		return err
	}

	// Perform seek
	newOffset, err := r.reader.Seek(offset, io.SeekStart)
	if err != nil {
		return fmt.Errorf("seek error to offset %d: %w", offset, err)
	}

	fs.Debugf(r.object, "Seek completed to offset %d (piece %d)",
		newOffset, newOffset/pieceSize)

	return nil
}

// clearPiecePriorities resets all piece priorities
func (r *enhancedReader) clearPiecePriorities(info *metainfo.Info) {
	for i := 0; i < info.NumPieces(); i++ {
		piece := r.object.entry.torrent.Piece(i)
		if !piece.State().Complete {
			piece.SetPriority(types.PiecePriorityNone)
		}
	}
}

// setPriorityWindow sets priorities for the read-ahead window
func (r *enhancedReader) setPriorityWindow(startPiece, numPieces int64) error {
	readAheadEnd := startPiece + int64(r.readAhead)
	if readAheadEnd >= numPieces {
		readAheadEnd = numPieces - 1
	}

	priorityWindow := min(5, int(readAheadEnd-startPiece+1))
	prioritizedCount := 0

	for i := startPiece; i <= readAheadEnd; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		if !piece.State().Complete {
			priority := types.PiecePriorityNormal
			if int(i-startPiece) < priorityWindow {
				priority = types.PiecePriorityReadahead
			}
			piece.SetPriority(priority)
			prioritizedCount++
		}
	}

	fs.Debugf(r.object, "Set priorities for %d pieces in window %d-%d",
		prioritizedCount, startPiece, readAheadEnd)

	return nil
}

// ensurePiecePriorities maintains proper piece priorities during reading
func (r *enhancedReader) ensurePiecePriorities(startPiece, endPiece int64) {
	info := r.object.entry.torrent.Info()
	if info == nil {
		return
	}

	readAheadEnd := endPiece + int64(r.readAhead)
	if readAheadEnd >= int64(info.NumPieces()) {
		readAheadEnd = int64(info.NumPieces() - 1)
	}

	// Check if priorities need updating
	if !r.needsPriorityUpdate(startPiece, endPiece, readAheadEnd) {
		return
	}

	// Update priorities
	r.updatePriorityWindow(startPiece, endPiece, readAheadEnd)
}

// needsPriorityUpdate checks if piece priorities need updating
func (r *enhancedReader) needsPriorityUpdate(startPiece, endPiece, readAheadEnd int64) bool {
	for i := startPiece; i <= readAheadEnd; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		state := piece.State()
		if !state.Complete {
			if i <= endPiece && state.Priority != types.PiecePriorityReadahead {
				return true
			} else if i > endPiece && state.Priority != types.PiecePriorityNormal {
				return true
			}
		}
	}
	return false
}

// updatePriorityWindow updates priorities for the current read window
func (r *enhancedReader) updatePriorityWindow(startPiece, endPiece, readAheadEnd int64) {
	var updates []string
	for i := startPiece; i <= readAheadEnd; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		if !piece.State().Complete {
			oldPriority := piece.State().Priority
			newPriority := types.PiecePriorityNormal
			if i <= endPiece {
				newPriority = types.PiecePriorityReadahead
			}
			if oldPriority != newPriority {
				piece.SetPriority(newPriority)
				updates = append(updates, fmt.Sprintf("%d: %v→%v", i, oldPriority, newPriority))
			}
		}
	}

	if len(updates) > 0 {
		fs.Debugf(r.object, "Updated priorities: %s", strings.Join(updates, ", "))
	}
}

// logTimeoutState logs detailed state information on timeout
func (r *enhancedReader) logTimeoutState(offset, startPiece, endPiece int64) {
	info := r.object.entry.torrent.Info()
	if info == nil {
		return
	}

	// Get piece states
	var pieceStates []string
	for i := startPiece; i <= endPiece; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		state := piece.State()
		pieceStates = append(pieceStates, fmt.Sprintf("piece_%d{complete=%v,priority=%v}",
			i, state.Complete, state.Priority))
	}

	stats := r.object.entry.torrent.Stats()
	fs.Debugf(r.object, "Timeout state at offset %d: pieces: %s, peers: %d (%d active)",
		offset, strings.Join(pieceStates, ", "), stats.TotalPeers, stats.ActivePeers)

	// Log details about pending pieces
	var pendingDetails []string
	for i := startPiece; i <= endPiece; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		if !piece.State().Complete {
			pendingDetails = append(pendingDetails, fmt.Sprintf("piece_%d(priority=%v)",
				i, piece.State().Priority))
		}
	}
	if len(pendingDetails) > 0 {
		fs.Debugf(r.object, "Pending pieces details: %s", strings.Join(pendingDetails, ", "))
	}
}

// logProgress logs detailed download progress information
func (r *enhancedReader) logProgress(bytesRead int) {
	now := time.Now()
	if now.Sub(r.lastLogUpdate) < time.Second*2 {
		return
	}

	torrent := r.object.entry.torrent
	stats := torrent.Stats()
	info := torrent.Info()
	if info == nil {
		return
	}

	currentOffset, _ := r.reader.Seek(0, io.SeekCurrent)
	currentPiece := currentOffset / r.pieceLength

	// Calculate completion stats
	completedPieces := 0
	var completedSize int64
	for i := 0; i < info.NumPieces(); i++ {
		if torrent.Piece(i).State().Complete {
			completedPieces++
			if i == info.NumPieces()-1 {
				lastPieceSize := r.object.Size() - int64(i)*r.pieceLength
				completedSize += lastPieceSize
			} else {
				completedSize += r.pieceLength
			}
		}
	}

	// Calculate current download rate
	elapsed := time.Since(r.lastLogUpdate)
	downloadRate := float64(bytesRead) / elapsed.Seconds()

	fs.Debugf(r.object, "Progress: pieces=%d/%d (%.1f%%, %s/%s), %.2f MB/s current, %d peers (%d active), at piece %d (offset %d)",
		completedPieces,
		info.NumPieces(),
		float64(completedPieces)/float64(info.NumPieces())*100,
		fs.SizeSuffix(completedSize),
		fs.SizeSuffix(r.object.Size()),
		downloadRate/1024/1024,
		stats.TotalPeers,
		stats.ActivePeers,
		currentPiece,
		currentOffset)

	// Log nearby piece states
	if currentPiece > 0 {
		r.logNearbyPieceStates(currentPiece, info)
	}

	r.lastLogUpdate = now
}

// logNearbyPieceStates logs the state of pieces near the current read position
func (r *enhancedReader) logNearbyPieceStates(currentPiece int64, info *metainfo.Info) {
	startLogPiece := currentPiece - 1
	endLogPiece := currentPiece + int64(r.readAhead)
	if endLogPiece >= int64(info.NumPieces()) {
		endLogPiece = int64(info.NumPieces() - 1)
	}

	var pieceStates []string
	for i := startLogPiece; i <= endLogPiece; i++ {
		piece := r.object.entry.torrent.Piece(int(i))
		state := piece.State()
		if !state.Complete {
			pieceStates = append(pieceStates, fmt.Sprintf("piece_%d(priority=%v)", i, state.Priority))
		}
	}

	if len(pieceStates) > 0 {
		fs.Debugf(r.object, "Nearby piece states: %s", strings.Join(pieceStates, ", "))
	}
}

// Close implements io.Closer
func (r *enhancedReader) Close() error {
	fs.Debugf(r.object, "Closing reader after reading %s", fs.SizeSuffix(r.bytesRead))
	return r.reader.Close()
}

// Helper functions

// calculateMediaReadAhead determines optimal read-ahead for media files
func calculateMediaReadAhead(fileSize, pieceLength int64, baseReadAhead int) int {
	piecesInFile := fileSize / pieceLength

	switch {
	case piecesInFile < 50: // Small files (< 100MB)
		return baseReadAhead
	case piecesInFile < 500: // Medium files (100MB - 1GB)
		return max(baseReadAhead*2, 64)
	default: // Large files (> 1GB)
		return max(baseReadAhead*4, 128)
	}
}

// isMediaFile checks if the file has a media extension
func isMediaFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	mediaExts := map[string]bool{
		".mp4": true, ".mkv": true, ".avi": true,
		".mov": true, ".wmv": true, ".flv": true,
		".m4v": true, ".mpg": true, ".mpeg": true,
		".ts": true, ".m2ts": true, ".vob": true,
		".webm": true, ".mts": true,
	}
	return mediaExts[ext]
}

// Helper types and functions
type readResult struct {
	n   int
	err error
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// Check interface implementation
var _ fs.Object = (*Object)(nil)
