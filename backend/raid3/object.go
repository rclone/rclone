// Package raid3 implements a backend that splits data across three remotes using byte-level striping
package raid3

// This file contains the Object and Directory types and their methods.
//
// It includes:
//   - Object type representing a striped object across three backends
//   - Directory type for directory entries
//   - particleObjectInfo wrapper for particle-specific metadata
//   - Object methods: Open, Size, ModTime, Hash, Remove, Update
//   - Particle reading and reconstruction logic with heal support
//   - Degraded mode handling (2/3 particles present) with automatic reconstruction

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"golang.org/x/sync/errgroup"
)

// particleObjectInfo wraps fs.ObjectInfo with a different size and optionally different remote name for particles
type particleObjectInfo struct {
	fs.ObjectInfo
	size   int64
	remote string // Override remote name (for parity files with suffix)
}

func (p *particleObjectInfo) Size() int64 {
	return p.size
}

func (p *particleObjectInfo) Remote() string {
	if p.remote != "" {
		return p.remote
	}
	return p.ObjectInfo.Remote()
}

// Hash returns an empty string to force the backend to recalculate hashes
// from the actual particle data instead of using the original file's hash
func (p *particleObjectInfo) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", nil
}

// Object represents a striped object
type Object struct {
	fs     *Fs
	remote string
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	// Get from even
	obj, err := o.fs.even.NewObject(ctx, o.remote)
	if err == nil {
		return obj.ModTime(ctx)
	}
	// Fallback to odd
	obj, err = o.fs.odd.NewObject(ctx, o.remote)
	if err == nil {
		return obj.ModTime(ctx)
	}
	return time.Now()
}

// Size returns the size of the reconstructed object
func (o *Object) Size() int64 {
	ctx := context.Background()
	// Try to fetch particles
	evenObj, errEven := o.fs.even.NewObject(ctx, o.remote)
	oddObj, errOdd := o.fs.odd.NewObject(ctx, o.remote)

	// Fast path: both data particles exist
	if errEven == nil && errOdd == nil {
		return evenObj.Size() + oddObj.Size()
	}

	// Otherwise try parity with either suffix
	// Determine which parity exists and whether original length is odd
	parityObj, errParity := o.fs.parity.NewObject(ctx, GetParityFilename(o.remote, true))
	isOddLength := false
	if errParity != nil {
		parityObj, errParity = o.fs.parity.NewObject(ctx, GetParityFilename(o.remote, false))
		if errParity != nil {
			return -1
		}
		isOddLength = false // .parity-el
	} else {
		isOddLength = true // .parity-ol
	}

	// If we have one data particle and parity, we can compute size
	if errEven == nil {
		// Missing odd: N = even + parity - (isOdd ? 1 : 0)
		if isOddLength {
			return evenObj.Size() + parityObj.Size() - 1
		}
		return evenObj.Size() + parityObj.Size()
	}
	if errOdd == nil {
		// Missing even: N = odd + parity (regardless of odd/even length)
		return oddObj.Size() + parityObj.Size()
	}

	return -1
}

// Hash returns the hash of the reconstructed object
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	// Must reconstruct the full file to calculate hash
	reader, err := o.Open(ctx)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	// Calculate hash of merged data
	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ty))
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(hasher, reader); err != nil {
		return "", err
	}

	return hasher.SumString(ty, false)
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	// Pre-flight health check: Enforce strict RAID 3 write policy
	// SetModTime is a write operation (modifies metadata)
	// Consistent with Put/Update/Move/Mkdir operations
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return err // Returns enhanced error with rebuild guidance
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return err
		}
		return obj.SetModTime(gCtx, t)
	})

	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return err
		}
		return obj.SetModTime(gCtx, t)
	})

	// Also set mod time on parity files
	g.Go(func() error {
		// Try both suffixes to find the parity file
		parityOdd := GetParityFilename(o.remote, true)
		obj, err := o.fs.parity.NewObject(gCtx, parityOdd)
		if err == nil {
			return obj.SetModTime(gCtx, t)
		}

		parityEven := GetParityFilename(o.remote, false)
		obj, err = o.fs.parity.NewObject(gCtx, parityEven)
		if err == nil {
			return obj.SetModTime(gCtx, t)
		}
		return nil // Ignore if parity not found
	})

	return g.Wait()
}

// Open opens the object for read, reconstructing from particles
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.fs.opt.UseStreaming {
		return o.openStreaming(ctx, options...)
	}
	return o.openBuffered(ctx, options...)
}

// openBuffered opens the object using the buffered (memory-based) approach
func (o *Object) openBuffered(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {

	// Attempt to open both data particles concurrently to avoid blocking
	type objResult struct {
		obj fs.Object
		err error
	}
	evenCh := make(chan objResult, 1)
	oddCh := make(chan objResult, 1)

	go func() {
		obj, err := o.fs.even.NewObject(ctx, o.remote)
		evenCh <- objResult{obj, err}
	}()

	go func() {
		obj, err := o.fs.odd.NewObject(ctx, o.remote)
		oddCh <- objResult{obj, err}
	}()

	// Wait for both results
	evenRes := <-evenCh
	oddRes := <-oddCh
	evenObj, errEven := evenRes.obj, evenRes.err
	oddObj, errOdd := oddRes.obj, oddRes.err

	var merged []byte
	if errEven == nil && errOdd == nil {
		// Validate sizes
		evenSize := evenObj.Size()
		oddSize := oddObj.Size()
		if !ValidateParticleSizes(evenSize, oddSize) {
			return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d", evenSize, oddSize)
		}
		// Read both and merge
		evenReader, err := evenObj.Open(ctx)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to open even particle: %w", o.fs.even.Name(), err)
		}
		oddReader, err := oddObj.Open(ctx)
		if err != nil {
			evenReader.Close()
			return nil, fmt.Errorf("%s: failed to open odd particle: %w", o.fs.odd.Name(), err)
		}
		evenData, err := io.ReadAll(evenReader)
		evenReader.Close()
		if err != nil {
			oddReader.Close()
			return nil, fmt.Errorf("%s: failed to read even particle: %w", o.fs.even.Name(), err)
		}
		oddData, err := io.ReadAll(oddReader)
		oddReader.Close()
		if err != nil {
			return nil, fmt.Errorf("%s: failed to read odd particle: %w", o.fs.odd.Name(), err)
		}
		merged, err = MergeBytes(evenData, oddData)
		if err != nil {
			return nil, fmt.Errorf("failed to merge particles: %w", err)
		}
	} else {
		// One particle missing - attempt reconstruction using parity
		// Find which parity exists and infer original length type
		parityNameOL := GetParityFilename(o.remote, true)
		parityObj, errParity := o.fs.parity.NewObject(ctx, parityNameOL)
		isOddLength := false
		if errParity == nil {
			isOddLength = true
		} else {
			parityNameEL := GetParityFilename(o.remote, false)
			parityObj, errParity = o.fs.parity.NewObject(ctx, parityNameEL)
			if errParity != nil {
				// Can't reconstruct - not enough particles
				if errEven != nil && errOdd != nil {
					return nil, fmt.Errorf("missing particles: even and odd unavailable and no parity found")
				}
				if errEven != nil {
					return nil, fmt.Errorf("missing even particle and no parity found: %w", errEven)
				}
				return nil, fmt.Errorf("missing odd particle and no parity found: %w", errOdd)
			}
			isOddLength = false
		}

		// Open known data + parity and reconstruct
		if errEven == nil {
			evenReader, err := evenObj.Open(ctx)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to open even particle: %w", o.fs.even.Name(), err)
			}
			parityReader, err := parityObj.Open(ctx)
			if err != nil {
				evenReader.Close()
				return nil, fmt.Errorf("%s: failed to open parity particle: %w", o.fs.parity.Name(), err)
			}
			evenData, err := io.ReadAll(evenReader)
			evenReader.Close()
			if err != nil {
				parityReader.Close()
				return nil, fmt.Errorf("%s: failed to read even particle: %w", o.fs.even.Name(), err)
			}
			parityData, err := io.ReadAll(parityReader)
			parityReader.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: failed to read parity particle: %w", o.fs.parity.Name(), err)
			}
			merged, err = ReconstructFromEvenAndParity(evenData, parityData, isOddLength)
			if err != nil {
				return nil, err
			}
			fs.Infof(o, "Reconstructed %s from even+parity (degraded mode)", o.remote)

			// Heal: queue missing odd particle for upload (if auto_heal enabled)
			if o.fs.opt.AutoHeal {
				_, oddData := SplitBytes(merged)
				o.fs.queueParticleUpload(o.remote, "odd", oddData, isOddLength)
			}

		} else if errOdd == nil {
			oddReader, err := oddObj.Open(ctx)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to open odd particle: %w", o.fs.odd.Name(), err)
			}
			parityReader, err := parityObj.Open(ctx)
			if err != nil {
				oddReader.Close()
				return nil, fmt.Errorf("%s: failed to open parity particle: %w", o.fs.parity.Name(), err)
			}
			oddData, err := io.ReadAll(oddReader)
			oddReader.Close()
			if err != nil {
				parityReader.Close()
				return nil, fmt.Errorf("%s: failed to read odd particle: %w", o.fs.odd.Name(), err)
			}
			parityData, err := io.ReadAll(parityReader)
			parityReader.Close()
			if err != nil {
				return nil, fmt.Errorf("%s: failed to read parity particle: %w", o.fs.parity.Name(), err)
			}
			merged, err = ReconstructFromOddAndParity(oddData, parityData, isOddLength)
			if err != nil {
				return nil, err
			}
			fs.Infof(o, "Reconstructed %s from odd+parity (degraded mode)", o.remote)

			// Heal: queue missing even particle for upload (if auto_heal enabled)
			if o.fs.opt.AutoHeal {
				evenData, _ := SplitBytes(merged)
				o.fs.queueParticleUpload(o.remote, "even", evenData, isOddLength)
			}

		} else {
			return nil, fmt.Errorf("cannot reconstruct: no data particle available")
		}
	}

	// Handle range/seek options on the merged data
	reader := bytes.NewReader(merged)

	// Parse range option if present
	var rangeStart, rangeEnd int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			rangeStart, rangeEnd = x.Start, x.End
		case *fs.SeekOption:
			rangeStart = x.Offset
			rangeEnd = -1
		}
	}

	// Apply range if specified
	if rangeStart > 0 || rangeEnd >= 0 {
		totalSize := int64(len(merged))

		// Handle negative start (from end)
		if rangeStart < 0 && rangeEnd >= 0 {
			rangeStart = totalSize - rangeEnd
			rangeEnd = -1
		}

		// Validate range
		if rangeStart < 0 {
			rangeStart = 0
		}
		if rangeStart > totalSize {
			rangeStart = totalSize
		}

		// Calculate end
		var rangedData []byte
		if rangeEnd < 0 || rangeEnd >= totalSize {
			// Read to end
			rangedData = merged[rangeStart:]
		} else {
			// Read specific range (end is inclusive)
			if rangeEnd >= rangeStart {
				rangedData = merged[rangeStart : rangeEnd+1]
			} else {
				rangedData = []byte{}
			}
		}

		reader = bytes.NewReader(rangedData)
	}

	// Return as ReadCloser (bytes.NewReader supports seeking)
	return io.NopCloser(reader), nil
}

// openStreaming opens the object using the streaming approach
func (o *Object) openStreaming(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {

	// Attempt to open both data particles concurrently to avoid blocking
	type objResult struct {
		obj fs.Object
		err error
	}
	evenCh := make(chan objResult, 1)
	oddCh := make(chan objResult, 1)

	go func() {
		obj, err := o.fs.even.NewObject(ctx, o.remote)
		evenCh <- objResult{obj, err}
	}()

	go func() {
		obj, err := o.fs.odd.NewObject(ctx, o.remote)
		oddCh <- objResult{obj, err}
	}()

	// Wait for both results
	evenRes := <-evenCh
	oddRes := <-oddCh
	evenObj, errEven := evenRes.obj, evenRes.err
	oddObj, errOdd := oddRes.obj, oddRes.err

	chunkSize := int(o.fs.opt.ChunkSize)

	if errEven == nil && errOdd == nil {
		// Normal mode: both particles available - use StreamMerger
		// Validate sizes
		evenSize := evenObj.Size()
		oddSize := oddObj.Size()
		if !ValidateParticleSizes(evenSize, oddSize) {
			return nil, fmt.Errorf("invalid particle sizes: even=%d, odd=%d", evenSize, oddSize)
		}

		// Extract range/seek options before opening particle readers
		// We don't want to pass range options to particle readers - they operate on merged coordinates
		var rangeStart, rangeEnd int64 = 0, -1
		filteredOptions := make([]fs.OpenOption, 0, len(options))
		for _, option := range options {
			switch x := option.(type) {
			case *fs.RangeOption:
				rangeStart, rangeEnd = x.Start, x.End
				// Don't pass range option to particle readers
			case *fs.SeekOption:
				rangeStart = x.Offset
				rangeEnd = -1
				// Don't pass seek option to particle readers
			default:
				filteredOptions = append(filteredOptions, option)
			}
		}

		evenReader, err := evenObj.Open(ctx, filteredOptions...)
		if err != nil {
			return nil, fmt.Errorf("%s: failed to open even particle: %w", o.fs.even.Name(), err)
		}

		oddReader, err := oddObj.Open(ctx, filteredOptions...)
		if err != nil {
			evenReader.Close()
			return nil, fmt.Errorf("%s: failed to open odd particle: %w", o.fs.odd.Name(), err)
		}

		merger := NewStreamMerger(evenReader, oddReader, chunkSize)

		// Handle range/seek options by wrapping the merger
		// For now, we'll apply range filtering on the merged stream (simple approach)
		// TODO: Optimize to apply range to particle readers directly (future enhancement)

		if rangeStart > 0 || rangeEnd >= 0 {
			// Apply range filtering on merged stream
			// This reads the entire stream but filters the output
			// Future optimization: apply range to particle readers directly
			return newRangeFilterReader(merger, rangeStart, rangeEnd, o.Size()), nil
		}

		return merger, nil
	} else {
		// Degraded mode: one particle missing - use StreamReconstructor
		// Find which parity exists and infer original length type
		parityNameOL := GetParityFilename(o.remote, true)
		parityObj, errParity := o.fs.parity.NewObject(ctx, parityNameOL)
		isOddLength := false
		if errParity == nil {
			isOddLength = true
		} else {
			parityNameEL := GetParityFilename(o.remote, false)
			parityObj, errParity = o.fs.parity.NewObject(ctx, parityNameEL)
			if errParity != nil {
				// Can't reconstruct - not enough particles
				if errEven != nil && errOdd != nil {
					return nil, fmt.Errorf("missing particles: even and odd unavailable and no parity found")
				}
				if errEven != nil {
					return nil, fmt.Errorf("missing even particle and no parity found: %w", errEven)
				}
				return nil, fmt.Errorf("missing odd particle and no parity found: %w", errOdd)
			}
			isOddLength = false
		}

		// Open known data + parity and reconstruct
		if errEven == nil {
			// Extract range/seek options before opening particle readers
			var rangeStart, rangeEnd int64 = 0, -1
			filteredOptions := make([]fs.OpenOption, 0, len(options))
			for _, option := range options {
				switch x := option.(type) {
				case *fs.RangeOption:
					rangeStart, rangeEnd = x.Start, x.End
					// Don't pass range option to particle readers
				case *fs.SeekOption:
					rangeStart = x.Offset
					rangeEnd = -1
					// Don't pass seek option to particle readers
				default:
					filteredOptions = append(filteredOptions, option)
				}
			}

			// Reconstruct from even + parity
			evenReader, err := evenObj.Open(ctx, filteredOptions...)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to open even particle: %w", o.fs.even.Name(), err)
			}

			parityReader, err := parityObj.Open(ctx, filteredOptions...)
			if err != nil {
				evenReader.Close()
				return nil, fmt.Errorf("%s: failed to open parity particle: %w", o.fs.parity.Name(), err)
			}

			reconstructor := NewStreamReconstructor(evenReader, parityReader, "even+parity", isOddLength, chunkSize)
			fs.Infof(o, "Reconstructed %s from even+parity (degraded mode, streaming)", o.remote)

			// Note: Heal operations would need to be adapted for streaming
			// For now, we skip auto-heal in streaming mode (can be added later)

			if rangeStart > 0 || rangeEnd >= 0 {
				return newRangeFilterReader(reconstructor, rangeStart, rangeEnd, o.Size()), nil
			}

			return reconstructor, nil

		} else if errOdd == nil {
			// Extract range/seek options before opening particle readers
			var rangeStart, rangeEnd int64 = 0, -1
			filteredOptions := make([]fs.OpenOption, 0, len(options))
			for _, option := range options {
				switch x := option.(type) {
				case *fs.RangeOption:
					rangeStart, rangeEnd = x.Start, x.End
					// Don't pass range option to particle readers
				case *fs.SeekOption:
					rangeStart = x.Offset
					rangeEnd = -1
					// Don't pass seek option to particle readers
				default:
					filteredOptions = append(filteredOptions, option)
				}
			}

			// Reconstruct from odd + parity
			oddReader, err := oddObj.Open(ctx, filteredOptions...)
			if err != nil {
				return nil, fmt.Errorf("%s: failed to open odd particle: %w", o.fs.odd.Name(), err)
			}

			parityReader, err := parityObj.Open(ctx, filteredOptions...)
			if err != nil {
				oddReader.Close()
				return nil, fmt.Errorf("%s: failed to open parity particle: %w", o.fs.parity.Name(), err)
			}

			reconstructor := NewStreamReconstructor(oddReader, parityReader, "odd+parity", isOddLength, chunkSize)
			fs.Infof(o, "Reconstructed %s from odd+parity (degraded mode, streaming)", o.remote)

			// Note: Heal operations would need to be adapted for streaming
			// For now, we skip auto-heal in streaming mode (can be added later)

			if rangeStart > 0 || rangeEnd >= 0 {
				return newRangeFilterReader(reconstructor, rangeStart, rangeEnd, o.Size()), nil
			}

			return reconstructor, nil

		} else {
			return nil, fmt.Errorf("cannot reconstruct: no data particle available")
		}
	}
}

// rangeFilterReader applies range filtering to a stream
type rangeFilterReader struct {
	reader     io.ReadCloser
	rangeStart int64
	rangeEnd   int64
	totalSize  int64
	pos        int64
}

func newRangeFilterReader(reader io.ReadCloser, rangeStart, rangeEnd int64, totalSize int64) *rangeFilterReader {
	// Use RangeOption.Decode semantics to interpret the range
	// This matches the standard rclone behavior
	opt := &fs.RangeOption{Start: rangeStart, End: rangeEnd}
	offset, limit := opt.Decode(totalSize)
	
	// Decode returns offset and limit (number of bytes to read)
	// Convert to rangeStart and rangeEnd (inclusive end)
	rangeStart = offset
	if limit >= 0 {
		rangeEnd = offset + limit - 1 // End is inclusive
	} else {
		rangeEnd = totalSize - 1 // Read to end
	}

	// Validate range
	if rangeStart < 0 {
		rangeStart = 0
	}
	if rangeStart > totalSize {
		rangeStart = totalSize
	}
	if rangeEnd < 0 {
		rangeEnd = totalSize - 1
	}
	if rangeEnd >= totalSize {
		rangeEnd = totalSize - 1
	}
	if rangeEnd < rangeStart {
		rangeEnd = rangeStart - 1 // Empty range
	}

	return &rangeFilterReader{
		reader:     reader,
		rangeStart: rangeStart,
		rangeEnd:   rangeEnd,
		totalSize:  totalSize,
		pos:        0,
	}
}

func (r *rangeFilterReader) Read(p []byte) (n int, err error) {
	// Skip bytes before rangeStart
	for r.pos < r.rangeStart {
		skip := int64(len(p))
		if skip > r.rangeStart-r.pos {
			skip = r.rangeStart - r.pos
		}
		buf := make([]byte, skip)
		n, err := r.reader.Read(buf)
		if err != nil && err != io.EOF {
			return 0, err
		}
		r.pos += int64(n)
		if err == io.EOF {
			return 0, io.EOF
		}
	}

	// If we're past the range, we're done
	if r.pos > r.rangeEnd {
		return 0, io.EOF
	}

	// Read data within range
	maxRead := int64(len(p))
	if r.pos+maxRead > r.rangeEnd+1 {
		maxRead = r.rangeEnd + 1 - r.pos
	}
	if maxRead <= 0 {
		return 0, io.EOF
	}

	buf := make([]byte, maxRead)
	n, err = r.reader.Read(buf)
	if n > 0 {
		copy(p, buf[:n])
		r.pos += int64(n)
	}

	return n, err
}

func (r *rangeFilterReader) Close() error {
	return r.reader.Close()
}

// Update updates the object
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	if o.fs.opt.UseStreaming {
		return o.updateStreaming(ctx, in, src, options...)
	}
	return o.updateBuffered(ctx, in, src, options...)
}

// updateBuffered updates the object using the buffered (memory-based) approach
func (o *Object) updateBuffered(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Pre-flight check: Enforce strict RAID 3 write policy
	// Fail immediately if any backend is unavailable to prevent corrupted updates
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("update blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = o.fs.disableRetriesForWrites(ctx)

	// Read data once
	data, err := io.ReadAll(in)
	if err != nil {
		return fmt.Errorf("failed to read data: %w", err)
	}

	// Split into even and odd bytes
	evenData, oddData := SplitBytes(data)

	// Calculate parity
	parityData := CalculateParity(evenData, oddData)

	// Determine if original data is odd length
	isOddLength := len(data)%2 == 1
	parityName := GetParityFilename(o.remote, isOddLength)

	// Two code paths: original approach (rollback disabled) or move-to-temp pattern (rollback enabled)
	if !o.fs.opt.Rollback {
		// Original approach: Update particles in place (no rollback)
		return o.updateInPlace(ctx, evenData, oddData, parityData, parityName, src, options...)
	}

	// Rollback enabled: Use move-to-temp pattern for rollback safety
	return o.updateWithRollback(ctx, evenData, oddData, parityData, parityName, src, options...)
}

// updateInPlace performs Update using the original approach (direct update on particle objects)
func (o *Object) updateInPlace(ctx context.Context, evenData, oddData, parityData []byte, parityName string, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Create wrapper ObjectInfo for each particle
	evenInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(evenData)),
	}
	oddInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(oddData)),
	}
	parityInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(parityData)),
		remote:     parityName,
	}

	g, gCtx := errgroup.WithContext(ctx)

	// Update even particle
	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("even particle not found: %w", err)
		}
		reader := bytes.NewReader(evenData)
		return obj.Update(gCtx, reader, evenInfo, options...)
	})

	// Update odd particle
	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("odd particle not found: %w", err)
		}
		reader := bytes.NewReader(oddData)
		return obj.Update(gCtx, reader, oddInfo, options...)
	})

	// Update or create parity particle
	g.Go(func() error {
		obj, err := o.fs.parity.NewObject(gCtx, parityName)
		if err != nil {
			// Parity doesn't exist, create it with Put
			reader := bytes.NewReader(parityData)
			_, err := o.fs.parity.Put(gCtx, reader, parityInfo, options...)
			return err
		}
		// Parity exists, update it
		reader := bytes.NewReader(parityData)
		return obj.Update(gCtx, reader, parityInfo, options...)
	})

	return g.Wait()
}

// updateWithRollback performs Update using move-to-temp pattern for rollback safety
func (o *Object) updateWithRollback(ctx context.Context, evenData, oddData, parityData []byte, parityName string, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	// Move original particles to temporary locations for rollback safety
	tempParticles := make(map[string]fs.Object)
	var tempMu sync.Mutex
	defer func() {
		if err != nil && len(tempParticles) > 0 {
			tempMu.Lock()
			temps := tempParticles
			tempMu.Unlock()
			if rollbackErr := o.fs.rollbackUpdate(ctx, temps); rollbackErr != nil {
				fs.Errorf(o.fs, "Rollback failed during Update: %v", rollbackErr)
			}
		} else if err == nil && len(tempParticles) > 0 {
			// Success: delete temp particles
			tempMu.Lock()
			temps := tempParticles
			tempMu.Unlock()
			for _, tempObj := range temps {
				if delErr := tempObj.Remove(ctx); delErr != nil {
					fs.Debugf(o.fs, "Failed to remove temp particle after successful update: %v", delErr)
				}
			}
		}
	}()

	g, gCtx := errgroup.WithContext(ctx)

	// Move original particles to temp locations
	// Even particle
	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("even particle not found: %w", err)
		}
		tempRemote := o.remote + ".tmp.even"
		moved, err := moveOrCopyParticleToTemp(gCtx, o.fs.even, obj, tempRemote)
		if err != nil {
			return fmt.Errorf("failed to move/copy even particle to temp: %w", err)
		}
		tempMu.Lock()
		tempParticles["even"] = moved
		tempMu.Unlock()
		return nil
	})

	// Odd particle
	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return fmt.Errorf("odd particle not found: %w", err)
		}
		tempRemote := o.remote + ".tmp.odd"
		moved, err := moveOrCopyParticleToTemp(gCtx, o.fs.odd, obj, tempRemote)
		if err != nil {
			return fmt.Errorf("failed to move/copy odd particle to temp: %w", err)
		}
		tempMu.Lock()
		tempParticles["odd"] = moved
		tempMu.Unlock()
		return nil
	})

	// Parity particle
	g.Go(func() error {
		obj, err := o.fs.parity.NewObject(gCtx, parityName)
		if err != nil {
			// Parity might not exist, that's ok
			return nil
		}
		tempRemote := parityName + ".tmp.parity"
		moved, err := moveOrCopyParticleToTemp(gCtx, o.fs.parity, obj, tempRemote)
		if err != nil {
			return fmt.Errorf("failed to move/copy parity particle to temp: %w", err)
		}
		tempMu.Lock()
		tempParticles["parity"] = moved
		tempMu.Unlock()
		return nil
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	// Verify that moves to temp succeeded by checking temp particles exist
	tempMu.Lock()
	tempCount := len(tempParticles)
	evenMoved := tempParticles["even"]
	oddMoved := tempParticles["odd"]
	tempMu.Unlock()

	if tempCount == 0 {
		return fmt.Errorf("move-to-temp failed: no particles moved to temp locations")
	}

	if evenMoved == nil || oddMoved == nil {
		return fmt.Errorf("move-to-temp failed: even=%v, odd=%v (at least even and odd must be moved)",
			evenMoved != nil, oddMoved != nil)
	}

	// Verify temp particles actually exist at temp locations
	if _, err := o.fs.even.NewObject(ctx, evenMoved.Remote()); err != nil {
		return fmt.Errorf("move-to-temp verification failed: even particle not found at temp location %s: %w", evenMoved.Remote(), err)
	}
	if _, err := o.fs.odd.NewObject(ctx, oddMoved.Remote()); err != nil {
		return fmt.Errorf("move-to-temp verification failed: odd particle not found at temp location %s: %w", oddMoved.Remote(), err)
	}

	// Create wrapper ObjectInfo for each particle
	// IMPORTANT: Use o.remote (the actual file path) not src.Remote() which might have suffixes
	// The test may pass ObjectInfo with a different remote name that should be ignored
	evenInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(evenData)),
		remote:     o.remote, // Use the object's remote, not src.Remote()
	}
	oddInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(oddData)),
		remote:     o.remote, // Use the object's remote, not src.Remote()
	}
	parityInfo := &particleObjectInfo{
		ObjectInfo: src,
		size:       int64(len(parityData)),
		remote:     parityName,
	}

	// Upload new particles at original location
	g, gCtx = errgroup.WithContext(ctx)

	var evenObjPut, oddObjPut fs.Object
	var putMu sync.Mutex

	// Update even
	g.Go(func() error {
		reader := bytes.NewReader(evenData)
		obj, err := o.fs.even.Put(gCtx, reader, evenInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload even particle: %w", err)
		}
		putMu.Lock()
		evenObjPut = obj
		putMu.Unlock()
		return nil
	})

	// Update odd
	g.Go(func() error {
		reader := bytes.NewReader(oddData)
		obj, err := o.fs.odd.Put(gCtx, reader, oddInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload odd particle: %w", err)
		}
		putMu.Lock()
		oddObjPut = obj
		putMu.Unlock()
		return nil
	})

	// Update or create parity
	g.Go(func() error {
		reader := bytes.NewReader(parityData)
		_, err := o.fs.parity.Put(gCtx, reader, parityInfo, options...)
		if err != nil {
			return fmt.Errorf("failed to upload parity particle: %w", err)
		}
		return nil
	})

	err = g.Wait()
	if err != nil {
		return err
	}

	// Get objects returned from Put for validation
	putMu.Lock()
	evenObj := evenObjPut
	oddObj := oddObjPut
	putMu.Unlock()

	if evenObj == nil || oddObj == nil {
		return fmt.Errorf("Put operations completed but objects are nil: even=%v, odd=%v", evenObj != nil, oddObj != nil)
	}

	// Verify files actually exist on filesystem by trying to read them
	// This ensures they're not just in-memory objects but actually persisted
	evenRC, errEvenOpen := evenObj.Open(ctx)
	if errEvenOpen != nil {
		return fmt.Errorf("validation failed: cannot open even particle after Put: %w", errEvenOpen)
	}
	evenRC.Close()

	oddRC, errOddOpen := oddObj.Open(ctx)
	if errOddOpen != nil {
		return fmt.Errorf("validation failed: cannot open odd particle after Put: %w", errOddOpen)
	}
	oddRC.Close()

	// Validate particle sizes
	if !ValidateParticleSizes(evenObj.Size(), oddObj.Size()) {
		return fmt.Errorf("update failed: invalid particle sizes (even=%d, odd=%d)",
			evenObj.Size(), oddObj.Size())
	}

	// Note: We intentionally do NOT verify the file through raid3.NewObject/Open here.
	// The validation above (opening particles directly and checking sizes) is sufficient
	// since we use o.remote (not src.Remote()) for particle paths. A final raid3
	// interface check was used during debugging but proved redundant once the root cause
	// (using src.Remote() instead of o.remote) was fixed. If issues arise in the future,
	// adding back a raid3 interface verification here can help diagnose path/visibility issues.

	return nil
}

// updateStreaming updates the object using the streaming approach with io.Pipe
// This mirrors the Get/Open pattern: streams data to Update() calls instead of calling Update() multiple times
func (o *Object) updateStreaming(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Pre-flight check: Enforce strict RAID 3 write policy
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("update blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = o.fs.disableRetriesForWrites(ctx)

	// Look up existing particle objects
	evenObj, err := o.fs.even.NewObject(ctx, o.remote)
	if err != nil {
		return fmt.Errorf("even particle not found: %w", err)
	}
	oddObj, err := o.fs.odd.NewObject(ctx, o.remote)
	if err != nil {
		return fmt.Errorf("odd particle not found: %w", err)
	}

	// Determine parity filename (try even-length first, then odd-length)
	// Store the old file's odd-length status based on which parity file exists
	oldParityName := GetParityFilename(o.remote, false)
	parityObj, err := o.fs.parity.NewObject(ctx, oldParityName)
	oldIsOddLength := false // Default to even-length
	if err != nil {
		oldParityName = GetParityFilename(o.remote, true)
		parityObj, err = o.fs.parity.NewObject(ctx, oldParityName)
		if err != nil {
			// Parity doesn't exist - will create with Put
			parityObj = nil
			oldParityName = "" // No old parity to clean up
		} else {
			oldIsOddLength = true // Found .parity-ol, so old file was odd-length
		}
	}
	// Initialize parityName - will be updated in goroutine if filename needs to change
	parityName := oldParityName

	// Track uploaded particles for rollback
	var uploadedParticles []fs.Object
	var err2 error
	defer func() {
		if err2 != nil && o.fs.opt.Rollback {
			particlesMap := make(map[string]fs.Object)
			if len(uploadedParticles) > 0 {
				particlesMap["even"] = uploadedParticles[0]
			}
			if len(uploadedParticles) > 1 {
				particlesMap["odd"] = uploadedParticles[1]
			}
			if len(uploadedParticles) > 2 {
				particlesMap["parity"] = uploadedParticles[2]
			}
			if rollbackErr := o.fs.rollbackUpdate(ctx, particlesMap); rollbackErr != nil {
				fs.Errorf(o.fs, "Rollback failed during Update (streaming): %v", rollbackErr)
			}
		}
	}()

	// Handle empty file case
	srcSize := src.Size()
	if srcSize == 0 {
		// Empty file - update with empty data
		evenInfo := createParticleInfo(src, "even", 0, false)
		err2 = evenObj.Update(ctx, bytes.NewReader(nil), evenInfo, options...)
		if err2 != nil {
			return fmt.Errorf("%s: failed to update even particle: %w", o.fs.even.Name(), err2)
		}
		uploadedParticles = append(uploadedParticles, evenObj)

		oddInfo := createParticleInfo(src, "odd", 0, false)
		err2 = oddObj.Update(ctx, bytes.NewReader(nil), oddInfo, options...)
		if err2 != nil {
			return fmt.Errorf("%s: failed to update odd particle: %w", o.fs.odd.Name(), err2)
		}
		uploadedParticles = append(uploadedParticles, oddObj)

		parityInfo := createParticleInfo(src, "parity", 0, false)
		parityInfo.remote = parityName
		if parityObj != nil {
			err2 = parityObj.Update(ctx, bytes.NewReader(nil), parityInfo, options...)
			if err2 != nil {
				return fmt.Errorf("%s: failed to update parity particle: %w", o.fs.parity.Name(), err2)
			}
			uploadedParticles = append(uploadedParticles, parityObj)
		} else {
			newParityObj, err := o.fs.parity.Put(ctx, bytes.NewReader(nil), parityInfo, options...)
			if err != nil {
				return fmt.Errorf("%s: failed to create parity particle: %w", o.fs.parity.Name(), err)
			}
			uploadedParticles = append(uploadedParticles, newParityObj)
		}

		return nil
	}

	// Configuration: Read 2MB chunks (produces ~1MB per particle)
	readChunkSize := int64(o.fs.opt.ChunkSize) * 2
	if readChunkSize < 2*1024*1024 {
		readChunkSize = 2 * 1024 * 1024 // Minimum 2MB
	}

	// Determine if file is odd-length from source size (if available)
	// If size is unknown (-1), we'll determine it during streaming
	isOddLength := srcSize > 0 && srcSize%2 == 1

	// Create pipes for streaming even, odd, and parity data
	evenPipeR, evenPipeW := io.Pipe()
	oddPipeR, oddPipeW := io.Pipe()
	parityPipeR, parityPipeW := io.Pipe()

	// Channel to communicate isOddLength from splitter to parity uploader
	// Only needed if size is unknown (-1)
	var isOddLengthCh chan bool
	if srcSize < 0 {
		isOddLengthCh = make(chan bool, 1)
		isOddLengthCh <- false // Default to even-length
	}

	// Track sizes for verification
	var totalEvenWritten, totalOddWritten int64

	// Use errgroup to coordinate input reading/splitting and Update operations
	g, gCtx := errgroup.WithContext(ctx)
	var uploadedMu sync.Mutex

	// Goroutine 1: Read input, split into even/odd/parity, write to pipes
	g.Go(func() error {
		defer evenPipeW.Close()
		defer oddPipeW.Close()
		defer parityPipeW.Close()

		// Buffer for reading chunks
		buffer := make([]byte, readChunkSize)

		for {
			// Read chunk from input
			n, readErr := in.Read(buffer)
			if n > 0 {
				// Split chunk into even and odd bytes
				evenData, oddData := SplitBytes(buffer[:n])
				parityData := CalculateParity(evenData, oddData)

				// Track sizes
				totalEvenWritten += int64(len(evenData))
				totalOddWritten += int64(len(oddData))

				// If size was unknown, detect odd-length from chunks
				if srcSize < 0 && len(evenData) > len(oddData) {
					// Update channel (non-blocking)
					select {
					case isOddLengthCh <- true:
					default:
						// Channel already has a value, drain and send new one
						select {
						case <-isOddLengthCh:
							isOddLengthCh <- true
						default:
						}
					}
				}

				// Write to pipes (these may block if readers are slow, which is fine)
				if _, err := evenPipeW.Write(evenData); err != nil {
					return fmt.Errorf("failed to write even data to pipe: %w", err)
				}
				if _, err := oddPipeW.Write(oddData); err != nil {
					return fmt.Errorf("failed to write odd data to pipe: %w", err)
				}
				if _, err := parityPipeW.Write(parityData); err != nil {
					return fmt.Errorf("failed to write parity data to pipe: %w", err)
				}
			}

			if readErr == io.EOF {
				break // End of input
			}
			if readErr != nil {
				return fmt.Errorf("failed to read input: %w", readErr)
			}
		}

		// If size was unknown, final check: evenWritten > oddWritten means odd-length
		if srcSize < 0 && totalEvenWritten > totalOddWritten {
			select {
			case isOddLengthCh <- true:
			default:
				select {
				case <-isOddLengthCh:
					isOddLengthCh <- true
				default:
				}
			}
		}

		return nil
	})

	// Goroutine 2: Update even particle (reads from evenPipeR)
	g.Go(func() error {
		defer evenPipeR.Close()
		evenInfo := createParticleInfo(src, "even", -1, isOddLength)
		err := evenObj.Update(gCtx, evenPipeR, evenInfo, options...)
		if err != nil {
			return fmt.Errorf("%s: failed to update even particle: %w", o.fs.even.Name(), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, evenObj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 3: Update odd particle (reads from oddPipeR)
	g.Go(func() error {
		defer oddPipeR.Close()
		oddInfo := createParticleInfo(src, "odd", -1, isOddLength)
		err := oddObj.Update(gCtx, oddPipeR, oddInfo, options...)
		if err != nil {
			return fmt.Errorf("%s: failed to update odd particle: %w", o.fs.odd.Name(), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, oddObj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 4: Update or create parity particle (reads from parityPipeR)
	g.Go(func() error {
		defer parityPipeR.Close()

		// Get new file's odd-length status - use source size if known, otherwise from channel
		newIsOddLength := isOddLength
		if srcSize < 0 && isOddLengthCh != nil {
			// Try to get from channel (non-blocking, use latest value)
			select {
			case newIsOddLength = <-isOddLengthCh:
			default:
				// Use default (even-length)
			}
		}

		// Determine if parity filename needs to change
		parityFilenameChanged := false
		if parityObj != nil && oldIsOddLength != newIsOddLength {
			// Old and new files have different odd-length status - need to change filename
			parityFilenameChanged = true
			// Delete old parity file
			if err := parityObj.Remove(gCtx); err != nil {
				return fmt.Errorf("%s: failed to remove old parity particle: %w", o.fs.parity.Name(), err)
			}
			parityObj = nil // Will create new one with Put
		}

		// Use new filename if it changed or if no old parity existed
		if parityFilenameChanged || oldParityName == "" {
			parityName = GetParityFilename(o.remote, newIsOddLength)
		}

		parityInfo := createParticleInfo(src, "parity", -1, newIsOddLength)
		parityInfo.remote = parityName

		if parityObj != nil {
			// Parity exists with correct filename, update it
			err := parityObj.Update(gCtx, parityPipeR, parityInfo, options...)
			if err != nil {
				return fmt.Errorf("%s: failed to update parity particle: %w", o.fs.parity.Name(), err)
			}
			uploadedMu.Lock()
			uploadedParticles = append(uploadedParticles, parityObj)
			uploadedMu.Unlock()
		} else {
			// Parity doesn't exist or was deleted, create it with Put
			newParityObj, err := o.fs.parity.Put(gCtx, parityPipeR, parityInfo, options...)
			if err != nil {
				return fmt.Errorf("%s: failed to create parity particle: %w", o.fs.parity.Name(), err)
			}
			uploadedMu.Lock()
			uploadedParticles = append(uploadedParticles, newParityObj)
			uploadedMu.Unlock()
		}
		return nil
	})

	// Wait for all goroutines to complete
	if err2 = g.Wait(); err2 != nil {
		return err2
	}

	// Verify sizes
	if err := verifyParticleSizes(ctx, o.fs, evenObj, oddObj, totalEvenWritten, totalOddWritten); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}

// Remove removes the object
func (o *Object) Remove(ctx context.Context) error {
	// Pre-flight check: Enforce strict RAID 3 delete policy
	// Fail immediately if any backend is unavailable to prevent partial deletes
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("delete blocked in degraded mode (RAID 3 policy): %w", err)
	}

	g, gCtx := errgroup.WithContext(ctx)

	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		return obj.Remove(gCtx)
	})

	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found
		}
		return obj.Remove(gCtx)
	})

	// Remove parity (try both suffixes)
	g.Go(func() error {
		parityOdd := GetParityFilename(o.remote, true)
		obj, err := o.fs.parity.NewObject(gCtx, parityOdd)
		if err == nil {
			return obj.Remove(gCtx)
		}

		parityEven := GetParityFilename(o.remote, false)
		obj, err = o.fs.parity.NewObject(gCtx, parityEven)
		if err == nil {
			return obj.Remove(gCtx)
		}
		return nil // Ignore if parity not found
	})

	return g.Wait()
}

// Directory represents a directory in the raid3 backend
type Directory struct {
	fs     *Fs
	remote string
}

// Fs returns the parent Fs
func (d *Directory) Fs() fs.Info {
	return d.fs
}

// String returns a description of the Directory
func (d *Directory) String() string {
	if d == nil {
		return "<nil>"
	}
	return d.remote
}

// Remote returns the remote path
func (d *Directory) Remote() string {
	return d.remote
}

// ModTime returns the modification time
func (d *Directory) ModTime(ctx context.Context) time.Time {
	return time.Now()
}

// Size returns the size (always 0 for directories)
func (d *Directory) Size() int64 {
	return 0
}

// Items returns the count of items in the directory
func (d *Directory) Items() int64 {
	return -1
}

// ID returns the internal ID of the directory
func (d *Directory) ID() string {
	return ""
}
