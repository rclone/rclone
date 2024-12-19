package torrent

import (
    "context"
    "fmt"
    "io"
    "sync"
    "sync/atomic"
    "time"

    "github.com/anacrolix/torrent"
    "github.com/anacrolix/torrent/types"
    "github.com/rclone/rclone/fs"
    "github.com/rclone/rclone/fs/hash"
)

const (
    defaultReadAhead int64 = 4 << 20
    largeFileReadAhead int64 = 16 << 20
    
    criticalWindow = 3
    prefetchWindow = 10
    
    priorityNow     = 255
    priorityHigh    = 192
    priorityNormal  = 128
    priorityLow     = 64
)

type Object struct {
    fs          *Fs
    virtualPath string
    torrentPath string
    size        int64
    modTime     time.Time
    sourcePath  string
}

func (o *Object) Fs() fs.Info                           { return o.fs }
func (o *Object) Remote() string                        { return o.virtualPath }
func (o *Object) ModTime(context.Context) time.Time     { return o.modTime }
func (o *Object) Size() int64                          { return o.size }
func (o *Object) Storable() bool                        { return false }
func (o *Object) String() string                        { return o.virtualPath }
func (o *Object) Hash(context.Context, hash.Type) (string, error) { return "", hash.ErrUnsupported }
func (o *Object) SetModTime(context.Context, time.Time) error { return fs.ErrorPermissionDenied }
func (o *Object) Remove(context.Context) error { return fs.ErrorPermissionDenied }
func (o *Object) Update(context.Context, io.Reader, fs.ObjectInfo, ...fs.OpenOption) error {
    return fs.ErrorPermissionDenied
}

type pieceWindow struct {
    start     int64
    end       int64 
    priority  int
    readCount int64
    lastRead  time.Time
}

type torrentReader struct {
    ctx         context.Context
    cancel      context.CancelFunc
    object      *Object
    file        *torrent.File
    reader      torrent.Reader
    startTime   time.Time
    closed      atomic.Bool
    mu          sync.Mutex
    offset      int64
    
    readAhead   int64
    pieceLength int64
    windows     []pieceWindow
    windowsMu   sync.Mutex
}


func (r *torrentReader) updatePiecePriorities(currentPiece int64) {
    r.windowsMu.Lock()
    defer r.windowsMu.Unlock()

    numPieces := r.file.Torrent().NumPieces()
    critical := currentPiece
    criticalEnd := min(critical+int64(criticalWindow), int64(numPieces))
    prefetch := criticalEnd
    prefetchEnd := min(prefetch+int64(prefetchWindow), int64(numPieces))

    for i := int64(0); i < int64(numPieces); i++ {
        piece := r.file.Torrent().Piece(int(i))
        if !piece.State().Complete {
            piece.SetPriority(types.PiecePriorityNone)
        }
    }

    for i := critical; i < criticalEnd; i++ {
        piece := r.file.Torrent().Piece(int(i))
        if !piece.State().Complete {
            piece.SetPriority(types.PiecePriorityNow)
            fs.Debugf(r.object, "Set critical priority for piece %d", i)
        }
    }

    for i := prefetch; i < prefetchEnd; i++ {
        piece := r.file.Torrent().Piece(int(i))
        if !piece.State().Complete {
            piece.SetPriority(types.PiecePriorityNormal)
            fs.Debugf(r.object, "Set prefetch priority for piece %d", i)
        }
    }

    // Update windows
    r.windows = []pieceWindow{
        {
            start:     critical,
            end:       criticalEnd,
            priority:  int(types.PiecePriorityNow),
            lastRead:  time.Now(),
        },
        {
            start:     prefetch,
            end:       prefetchEnd,
            priority:  int(types.PiecePriorityNormal),
            lastRead:  time.Now(),
        },
    }
}

func (r *torrentReader) logProgress() {
    stats := r.file.Torrent().Stats()
    bytesCompleted := r.file.BytesCompleted()
    progress := float64(bytesCompleted) / float64(r.file.Length()) * 100

    fs.Debugf(r.object, "Progress: %.1f%%, Active peers: %d, Total peers: %d, Current offset: %d/%d",
        progress,
        stats.ActivePeers,
        stats.TotalPeers,
        atomic.LoadInt64(&r.offset),
        r.file.Length())

    if r.windows != nil {
        r.windowsMu.Lock()
        for _, window := range r.windows {
            completed := 0
            for i := window.start; i < window.end; i++ {
                if r.file.Torrent().Piece(int(i)).State().Complete {
                    completed++
                }
            }
            fs.Debugf(r.object, "Window [%d-%d]: %d/%d pieces complete, priority %d",
                window.start, window.end, completed, window.end-window.start, window.priority)
        }
        r.windowsMu.Unlock()
    }
}

func (r *torrentReader) Read(p []byte) (n int, err error) {
    if r.closed.Load() {
        return 0, io.ErrClosedPipe
    }

    r.mu.Lock()
    reader := r.reader
    r.mu.Unlock()

    if reader == nil {
        return 0, io.ErrClosedPipe
    }

    currentPos := atomic.LoadInt64(&r.offset)
    currentPiece := currentPos / r.pieceLength

    needsUpdate := false
    r.windowsMu.Lock()
    for i := range r.windows {
        if currentPiece >= r.windows[i].end {
            needsUpdate = true
            break
        }
    }
    r.windowsMu.Unlock()

    if needsUpdate {
        r.updatePiecePriorities(currentPiece)
    }

    // Perform the read
    readCtx, cancel := context.WithTimeout(r.ctx, 30*time.Second)
    defer cancel()

    readDone := make(chan struct {
        n   int
        err error
    }, 1)

    go func() {
        n, err := reader.Read(p)
        readDone <- struct {
            n   int
            err error
        }{n, err}
    }()

    select {
    case result := <-readDone:
        if result.err == nil {
            atomic.AddInt64(&r.offset, int64(result.n))
            
            // Log progress periodically
            if time.Now().Unix()%5 == 0 {
                r.logProgress()
            }
        }
        return result.n, result.err
    case <-readCtx.Done():
        return 0, readCtx.Err()
    }
}

func (r *torrentReader) Seek(offset int64, whence int) (int64, error) {
    if r.closed.Load() {
        return 0, io.ErrClosedPipe
    }

    r.mu.Lock()
    defer r.mu.Unlock()

    if r.reader == nil {
        return 0, io.ErrClosedPipe
    }

    var abs int64
    switch whence {
    case io.SeekStart:
        abs = offset
    case io.SeekCurrent:
        abs = atomic.LoadInt64(&r.offset) + offset
    case io.SeekEnd:
        abs = r.file.Length() + offset
    default:
        return 0, fmt.Errorf("invalid whence: %d", whence)
    }

    if abs < 0 {
        return 0, fmt.Errorf("negative seek position: %d", abs)
    }
    if abs > r.file.Length() {
        return 0, fmt.Errorf("seek beyond end: %d > %d", abs, r.file.Length())
    }

    newPiece := abs / r.pieceLength
    r.updatePiecePriorities(newPiece)

    pos, err := r.reader.Seek(abs, io.SeekStart)
    if err == nil {
        atomic.StoreInt64(&r.offset, pos)
        fs.Debugf(r.object, "Seeked to offset %d (piece %d)", pos, newPiece)
    }

    return pos, err
}

func (r *torrentReader) RangeSeek(ctx context.Context, offset int64, whence int, length int64) (int64, error) {
    newReader := r.file.NewReader()
    
    pos, err := newReader.Seek(offset, whence)
    if err != nil {
        newReader.Close()
        return 0, err
    }

    r.mu.Lock()
    if oldReader := r.reader; oldReader != nil {
        oldReader.Close()
    }
    r.reader = newReader
    atomic.StoreInt64(&r.offset, pos)
    r.mu.Unlock()

    newPiece := pos / r.pieceLength
    r.updatePiecePriorities(newPiece)

    return pos, nil
}

func (r *torrentReader) Close() error {
    if !r.closed.CompareAndSwap(false, true) {
        return nil
    }

    r.cancel()
    r.mu.Lock()
    reader := r.reader
    r.reader = nil
    r.mu.Unlock()

    fs.Debugf(r.object, "Closed reader after %v", time.Since(r.startTime))

    if reader != nil {
        return reader.Close()
    }
    return nil
}

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
    fs.Debugf(o, "Opening file: %q", o.virtualPath)

    t, err := o.fs.getTorrent(o.sourcePath)
    if err != nil {
        return nil, fmt.Errorf("failed to load torrent: %w", err)
    }

    var targetFile *torrent.File
    for _, file := range t.Files() {
        if o.torrentPath == file.DisplayPath() {
            targetFile = file
            break
        }
    }

    if targetFile == nil {
        return nil, fmt.Errorf("file not found in torrent: %s", o.virtualPath)
    }

    ctx, cancel := context.WithCancel(ctx)
    tReader := targetFile.NewReader()
    readAhead := defaultReadAhead

    if targetFile.Length() > 1<<30 { // > 1GB
        readAhead = largeFileReadAhead
    }
    tReader.SetReadahead(readAhead)

    tr := &torrentReader{
        ctx:         ctx,
        cancel:      cancel,
        object:      o,
        file:        targetFile,
        reader:      tReader,
        startTime:   time.Now(),
        readAhead:   readAhead,
        pieceLength: int64(targetFile.Torrent().Info().PieceLength),
    }

    // Initialize first piece window
    tr.updatePiecePriorities(0)

    // Handle initial seek
    for _, option := range options {
        switch opt := option.(type) {
        case *fs.SeekOption:
            _, err = tr.Seek(opt.Offset, io.SeekStart)
            if err != nil {
                tr.Close()
                return nil, fmt.Errorf("initial seek failed: %w", err)
            }
        }
    }

    fs.Debugf(o, "Opened with read-ahead size: %d bytes", readAhead)
    return tr, nil
}

// Helper functions
func min(a, b int64) int64 {
    if a < b {
        return a
    }
    return b
}

func max(a, b int) int {
    if a > b {
        return a
    }
    return b
}

// Interface checks
var (
    _ io.Reader     = (*torrentReader)(nil)
    _ io.Closer     = (*torrentReader)(nil)
    _ io.Seeker     = (*torrentReader)(nil)
    _ fs.RangeSeeker = (*torrentReader)(nil)
)