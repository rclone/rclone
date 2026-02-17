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
	"bufio"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"path"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
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

// Metadata returns metadata from the wrapped ObjectInfo
// This allows backends to extract metadata during Put operations
func (p *particleObjectInfo) Metadata(ctx context.Context) (fs.Metadata, error) {
	if do, ok := p.ObjectInfo.(fs.Metadataer); ok {
		return do.Metadata(ctx)
	}
	return nil, nil
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

// readFooterFromParticle reads the last FooterSize bytes from a particle and parses the footer.
// Returns nil if the object is too small or parse fails.
func (o *Object) readFooterFromParticle(ctx context.Context, obj fs.Object) (*Footer, error) {
	size := obj.Size()
	if size < FooterSize {
		return nil, fmt.Errorf("particle too small for footer: %d", size)
	}
	rd, err := obj.Open(ctx, &fs.RangeOption{Start: size - FooterSize, End: size - 1})
	if err != nil {
		return nil, err
	}
	defer func() { _ = rd.Close() }()
	buf := make([]byte, FooterSize)
	if _, err := io.ReadFull(rd, buf); err != nil {
		return nil, err
	}
	return ParseFooter(buf)
}

// ModTime returns the modification time (from footer).
// In degraded mode (nil backend), skips that backend and tries the others.
func (o *Object) ModTime(ctx context.Context) time.Time {
	for _, fsBackend := range []fs.Fs{o.fs.even, o.fs.odd, o.fs.parity} {
		if fsBackend == nil {
			continue
		}
		obj, err := fsBackend.NewObject(ctx, o.remote)
		if err == nil {
			if ft, err := o.readFooterFromParticle(ctx, obj); err == nil {
				return time.Unix(ft.Mtime, 0)
			}
		}
	}
	return time.Now()
}

// Size returns the size of the reconstructed object.
// In degraded mode (nil backend), skips that backend and uses the first available particle.
// Note: This method doesn't accept a context parameter as it matches the rclone fs.Object interface.
func (o *Object) Size() int64 {
	ctx := context.Background() // Interface limitation: Size() doesn't accept context
	for _, fsBackend := range []fs.Fs{o.fs.even, o.fs.odd, o.fs.parity} {
		if fsBackend == nil {
			continue
		}
		obj, err := fsBackend.NewObject(ctx, o.remote)
		if err == nil {
			if ft, err := o.readFooterFromParticle(ctx, obj); err == nil {
				return ft.ContentLength
			}
		}
	}
	return -1
}

// Hash returns the hash of the reconstructed object.
// In degraded mode (nil backend), skips that backend and uses the first available particle.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	for _, fsBackend := range []fs.Fs{o.fs.even, o.fs.odd, o.fs.parity} {
		if fsBackend == nil {
			continue
		}
		obj, err := fsBackend.NewObject(ctx, o.remote)
		if err == nil {
			ft, err := o.readFooterFromParticle(ctx, obj)
			if err == nil {
				switch ty {
				case hash.MD5:
					return hex.EncodeToString(ft.MD5[:]), nil
				case hash.SHA256:
					return hex.EncodeToString(ft.SHA256[:]), nil
				}
				break // Footer doesn't have other types, fall through to full stream
			}
		}
	}

	reportedSize := o.Size()
	// When Size() is -1 (e.g. backend inconsistency or race during verify), still compute hash from stream
	// so copy verification can succeed. We skip size validation in that case.
	if reportedSize < 0 {
		fs.Debugf(o, "Hash: Size() returned -1, hashing from stream without size check")
	}

	reader, err := o.Open(ctx)
	if err != nil {
		return "", err
	}
	defer fs.CheckClose(reader, &err)

	hasher, err := hash.NewMultiHasherTypes(hash.NewHashSet(ty))
	if err != nil {
		return "", err
	}

	bytesRead, err := io.Copy(hasher, reader)
	if err != nil {
		return "", err
	}

	if reportedSize > 0 && bytesRead == 0 {
		return "", formatOperationError("hash calculation failed", fmt.Sprintf("read 0 bytes but Size() reports %d bytes - possible corruption", reportedSize), nil)
	}
	if reportedSize > 0 && bytesRead != reportedSize {
		fs.Debugf(o, "Hash calculation read %d bytes but Size() reports %d bytes", bytesRead, reportedSize)
	}

	return hasher.SumString(ty, false)
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// updateFooterMtime rewrites the 90-byte footer in each particle so that ModTime() returns t.
// Used by SetModTime and by SetMetadata when metadata["mtime"] is set.
func (o *Object) updateFooterMtime(ctx context.Context, t time.Time) error {
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return err
	}
	g, gCtx := errgroup.WithContext(ctx)
	backends := []fs.Fs{o.fs.even, o.fs.odd, o.fs.parity}
	for _, backend := range backends {
		backend := backend
		g.Go(func() error {
			obj, err := backend.NewObject(gCtx, o.remote)
			if err != nil {
				return err
			}
			size := obj.Size()
			if size < FooterSize {
				return nil
			}
			ft, err := o.readFooterFromParticle(gCtx, obj)
			if err != nil {
				return err
			}
			newFt := *ft
			newFt.Mtime = t.Unix()
			fb, err := newFt.MarshalBinary()
			if err != nil {
				return err
			}
			var combined io.Reader
			if size <= FooterSize {
				combined = bytes.NewReader(fb)
			} else {
				payloadReader, err := obj.Open(gCtx, &fs.RangeOption{Start: 0, End: size - FooterSize - 1})
				if err != nil {
					return err
				}
				payload, err := io.ReadAll(payloadReader)
				_ = payloadReader.Close()
				if err != nil {
					return err
				}
				combined = io.MultiReader(bytes.NewReader(payload), bytes.NewReader(fb))
			}
			info := object.NewStaticObjectInfo(o.remote, t, size, true, nil, nil)
			return obj.Update(gCtx, combined, info)
		})
	}
	return g.Wait()
}

// SetModTime sets the modification time by rewriting the 90-byte footer in each particle
// so that ModTime() (which reads from the footer) returns the new time.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return o.updateFooterMtime(ctx, t)
}

// Metadata returns metadata for the object.
// In degraded mode (nil backend), skips that backend.
// It should return nil if there is no Metadata
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	// Read metadata from even and odd particles and merge (parity doesn't contain file metadata)
	var mergedMetadata fs.Metadata
	for _, fsBackend := range []fs.Fs{o.fs.even, o.fs.odd} {
		if fsBackend == nil {
			continue
		}
		obj, err := fsBackend.NewObject(ctx, o.remote)
		if err == nil {
			if do, ok := obj.(fs.Metadataer); ok {
				meta, err := do.Metadata(ctx)
				if err == nil && meta != nil {
					if mergedMetadata == nil {
						mergedMetadata = make(fs.Metadata)
					}
					mergedMetadata.Merge(meta)
				}
			}
		}
	}
	// Return merged metadata (or nil if no metadata found)
	return mergedMetadata, nil
}

// SetMetadata sets metadata for the object
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	// Input validation
	if err := validateContext(ctx, "setmetadata"); err != nil {
		return err
	}
	if o == nil {
		return formatOperationError("setmetadata failed", "object cannot be nil", nil)
	}
	if err := validateRemote(o.remote, "setmetadata"); err != nil {
		return err
	}
	if metadata == nil {
		return formatOperationError("setmetadata failed", "metadata cannot be nil", nil)
	}

	// Pre-flight check: Enforce strict RAID 3 write policy
	// SetMetadata is a metadata write operation
	// Consistent with Object.SetModTime, Put, Update, Move operations
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return formatOperationError("setmetadata blocked in degraded mode (RAID 3 policy)", "", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = o.fs.disableRetriesForWrites(ctx)

	// If mtime is in metadata, update footer so ModTime() returns it
	if s, ok := metadata["mtime"]; ok && s != "" {
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			if err := o.updateFooterMtime(ctx, t); err != nil {
				return err
			}
		}
	}

	// Set metadata on all three backends that support it
	// Use errgroup to collect errors from all backends
	g, gCtx := errgroup.WithContext(ctx)

	// Set metadata on even particle
	g.Go(func() error {
		obj, err := o.fs.even.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found (degraded mode)
		}
		if do, ok := obj.(fs.SetMetadataer); ok {
			err := do.SetMetadata(gCtx, metadata)
			if err != nil {
				return formatBackendError(o.fs.even, "setmetadata failed", fmt.Sprintf("remote %q", o.remote), err)
			}
		}
		return nil
	})

	// Set metadata on odd particle
	g.Go(func() error {
		obj, err := o.fs.odd.NewObject(gCtx, o.remote)
		if err != nil {
			return nil // Ignore if not found (degraded mode)
		}
		if do, ok := obj.(fs.SetMetadataer); ok {
			err := do.SetMetadata(gCtx, metadata)
			if err != nil {
				return formatBackendError(o.fs.odd, "setmetadata failed", fmt.Sprintf("remote %q", o.remote), err)
			}
		}
		return nil
	})

	g.Go(func() error {
		obj, err := o.fs.parity.NewObject(gCtx, o.remote)
		if err == nil {
			if do, ok := obj.(fs.SetMetadataer); ok {
				err := do.SetMetadata(gCtx, metadata)
				if err != nil {
					return formatBackendError(o.fs.parity, "setmetadata failed", fmt.Sprintf("remote %q", o.remote), err)
				}
			}
		}
		return nil
	})

	return g.Wait()
}

// Open opens the object for read, reconstructing from particles
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// Input validation
	if err := validateContext(ctx, "open"); err != nil {
		return nil, err
	}
	if o == nil {
		return nil, formatOperationError("open failed", "object cannot be nil", nil)
	}
	if err := validateRemote(o.remote, "open"); err != nil {
		return nil, err
	}

	return o.openStreaming(ctx, options...)
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
		evenSize := evenObj.Size()
		oddSize := oddObj.Size()
		if evenSize < FooterSize || oddSize < FooterSize {
			return nil, formatOperationError("open failed", fmt.Sprintf("particle too small for footer: even=%d, odd=%d for remote %q", evenSize, oddSize, o.remote), nil)
		}
		evenPayload := evenSize - FooterSize
		oddPayload := oddSize - FooterSize
		if !ValidateParticleSizes(evenPayload, oddPayload) {
			return nil, formatOperationError("open failed",
				fmt.Sprintf("invalid particle sizes: even=%d, odd=%d for remote %q (expected even=odd or even=odd+1; object may be corrupted from a failed or partial upload; try re-uploading or remove the object and re-upload)", evenPayload, oddPayload, o.remote), nil)
		}

		expectedSize := evenPayload + oddPayload
		reportedSize := o.Size()
		if reportedSize != expectedSize && reportedSize >= 0 {
			fs.Debugf(o, "Size mismatch: Size()=%d, expected from particles=%d (even=%d, odd=%d)",
				reportedSize, expectedSize, evenPayload, oddPayload)
		}

		if expectedSize > 0 && (evenPayload < 0 || oddPayload < 0) {
			return nil, fmt.Errorf("particles report invalid sizes: even=%d, odd=%d (expected > 0)", evenPayload, oddPayload)
		}

		// Empty file: return a reader that yields 0 bytes (no need to open particles; otherwise we'd read the footer as content)
		if expectedSize == 0 {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}

		// Read footer from one particle to get compression type before opening payload streams
		var footer *Footer
		footerReader, ferr := evenObj.Open(ctx, &fs.RangeOption{Start: evenSize - FooterSize, End: evenSize - 1})
		if ferr == nil {
			footerBuf := make([]byte, FooterSize)
			_, _ = io.ReadFull(footerReader, footerBuf)
			_ = footerReader.Close()
			footer, _ = ParseFooter(footerBuf)
		}
		compression := CompressionNone
		if footer != nil {
			compression = footer.Compression
		}

		// Extract range/seek options before opening particle readers
		var rangeStart, rangeEnd int64 = 0, -1
		filteredOptions := make([]fs.OpenOption, 0, len(options))
		for _, option := range options {
			switch x := option.(type) {
			case *fs.RangeOption:
				rangeStart, rangeEnd = x.Start, x.End
			case *fs.SeekOption:
				rangeStart = x.Offset
				rangeEnd = -1
			default:
				filteredOptions = append(filteredOptions, option)
			}
		}
		evenOpenOpts := filteredOptions
		oddOpenOpts := filteredOptions
		if evenPayload > 0 {
			evenOpenOpts = append([]fs.OpenOption{}, filteredOptions...)
			evenOpenOpts = append(evenOpenOpts, &fs.RangeOption{Start: 0, End: evenPayload - 1})
			oddOpenOpts = append([]fs.OpenOption{}, filteredOptions...)
			oddOpenOpts = append(oddOpenOpts, &fs.RangeOption{Start: 0, End: oddPayload - 1})
		}

		evenReader, err := evenObj.Open(ctx, evenOpenOpts...)
		if err != nil {
			return nil, formatParticleError(o.fs.even, "even", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		oddReader, err := oddObj.Open(ctx, oddOpenOpts...)
		if err != nil {
			_ = evenReader.Close()
			return nil, formatParticleError(o.fs.odd, "odd", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		merger := NewStreamMerger(evenReader, oddReader, chunkSize)

		// Decompress if object was stored with compression
		out, err := newDecompressingReadCloser(merger, compression)
		if err != nil {
			_ = merger.Close()
			return nil, err
		}

		// Handle range/seek options by wrapping the merger
		// For now, we'll apply range filtering on the merged stream (simple approach)
		// TODO: Optimize to apply range to particle readers directly (future enhancement)

		if rangeStart > 0 || rangeEnd >= 0 {
			// Apply range filtering on merged stream
			// This reads the entire stream but filters the output
			// Future optimization: apply range to particle readers directly
			return newRangeFilterReader(out, rangeStart, rangeEnd, o.Size()), nil
		}

		return out, nil
	}
	// Degraded mode: one particle missing - use StreamReconstructor
	var parityObj fs.Object
	var errParity error
	var isOddLength bool
	var degradedFooter *Footer
	parityObj, errParity = o.fs.parity.NewObject(ctx, o.remote)
	if errParity != nil {
		if errEven != nil && errOdd != nil {
			return nil, fmt.Errorf("missing particles: even and odd unavailable and no parity found")
		}
		if errEven != nil {
			return nil, fmt.Errorf("missing even particle and no parity found: %w", errEven)
		}
		return nil, fmt.Errorf("missing odd particle and no parity found: %w", errOdd)
	}
	var sizeForFooter int64
	var objForFooter fs.Object
	if errEven == nil {
		objForFooter = evenObj
		sizeForFooter = evenObj.Size()
	} else {
		objForFooter = oddObj
		sizeForFooter = oddObj.Size()
	}
	if sizeForFooter >= FooterSize {
		footerReader, ferr := objForFooter.Open(ctx, &fs.RangeOption{Start: sizeForFooter - FooterSize, End: sizeForFooter - 1})
		if ferr == nil {
			footerBuf := make([]byte, FooterSize)
			_, _ = io.ReadFull(footerReader, footerBuf)
			_ = footerReader.Close()
			if ft, parseErr := ParseFooter(footerBuf); parseErr == nil {
				isOddLength = ft.ContentLength%2 == 1
				degradedFooter = ft
			}
		}
	}
	degradedCompression := CompressionNone
	if degradedFooter != nil {
		degradedCompression = degradedFooter.Compression
	}

	// Open known data + parity and reconstruct
	if errEven == nil {
		evenSize := evenObj.Size()
		paritySize := parityObj.Size()
		if evenSize < FooterSize || paritySize < FooterSize {
			return nil, formatOperationError("open failed", fmt.Sprintf("particle too small for footer: even=%d, parity=%d for remote %q", evenSize, paritySize, o.remote), nil)
		}
		evenPayload := evenSize - FooterSize
		parityPayload := paritySize - FooterSize
		if evenPayload < 0 || parityPayload < 0 {
			return nil, formatOperationError("open failed", fmt.Sprintf("invalid particle sizes for reconstruction: even=%d, parity=%d for remote %q", evenPayload, parityPayload, o.remote), nil)
		}

		var rangeStart, rangeEnd int64 = 0, -1
		filteredOptions := make([]fs.OpenOption, 0, len(options))
		for _, option := range options {
			switch x := option.(type) {
			case *fs.RangeOption:
				rangeStart, rangeEnd = x.Start, x.End
			case *fs.SeekOption:
				rangeStart = x.Offset
				rangeEnd = -1
			default:
				filteredOptions = append(filteredOptions, option)
			}
		}
		if evenPayload > 0 {
			filteredOptions = append(append([]fs.OpenOption{}, filteredOptions...), &fs.RangeOption{Start: 0, End: evenPayload - 1})
		}

		// Reconstruct from even + parity
		evenReader, err := evenObj.Open(ctx, filteredOptions...)
		if err != nil {
			return nil, formatParticleError(o.fs.even, "even", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		parityOpts := filteredOptions
		if parityPayload > 0 {
			parityOpts = append([]fs.OpenOption{}, filteredOptions...)
			parityOpts = append(parityOpts, &fs.RangeOption{Start: 0, End: parityPayload - 1})
		}
		parityReader, err := parityObj.Open(ctx, parityOpts...)
		if err != nil {
			_ = evenReader.Close()
			return nil, formatParticleError(o.fs.parity, "parity", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		reconstructor := NewStreamReconstructor(evenReader, parityReader, "even+parity", isOddLength, chunkSize)
		fs.Infof(o, "Reconstructed %s from even+parity (degraded mode, streaming)", o.remote)

		out, err := newDecompressingReadCloser(reconstructor, degradedCompression)
		if err != nil {
			_ = reconstructor.Close()
			return nil, err
		}

		if rangeStart > 0 || rangeEnd >= 0 {
			return newRangeFilterReader(out, rangeStart, rangeEnd, o.Size()), nil
		}

		return out, nil
	}
	if errOdd == nil {
		oddSize := oddObj.Size()
		paritySize := parityObj.Size()
		if oddSize < FooterSize || paritySize < FooterSize {
			return nil, formatOperationError("open failed", fmt.Sprintf("particle too small for footer: odd=%d, parity=%d for remote %q", oddSize, paritySize, o.remote), nil)
		}
		oddPayload := oddSize - FooterSize
		parityPayload := paritySize - FooterSize
		if oddPayload < 0 || parityPayload < 0 {
			return nil, formatOperationError("open failed", fmt.Sprintf("invalid particle sizes for reconstruction: odd=%d, parity=%d for remote %q", oddPayload, parityPayload, o.remote), nil)
		}

		var rangeStart, rangeEnd int64 = 0, -1
		filteredOptions := make([]fs.OpenOption, 0, len(options))
		for _, option := range options {
			switch x := option.(type) {
			case *fs.RangeOption:
				rangeStart, rangeEnd = x.Start, x.End
			case *fs.SeekOption:
				rangeStart = x.Offset
				rangeEnd = -1
			default:
				filteredOptions = append(filteredOptions, option)
			}
		}
		if oddPayload > 0 {
			filteredOptions = append(append([]fs.OpenOption{}, filteredOptions...), &fs.RangeOption{Start: 0, End: oddPayload - 1})
		}

		// Reconstruct from odd + parity
		oddReader, err := oddObj.Open(ctx, filteredOptions...)
		if err != nil {
			return nil, formatParticleError(o.fs.odd, "odd", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		parityOpts := filteredOptions
		if parityPayload > 0 {
			parityOpts = append([]fs.OpenOption{}, filteredOptions...)
			parityOpts = append(parityOpts, &fs.RangeOption{Start: 0, End: parityPayload - 1})
		}
		parityReader, err := parityObj.Open(ctx, parityOpts...)
		if err != nil {
			_ = oddReader.Close()
			return nil, formatParticleError(o.fs.parity, "parity", "open failed", fmt.Sprintf("remote %q", o.remote), err)
		}

		reconstructor := NewStreamReconstructor(oddReader, parityReader, "odd+parity", isOddLength, chunkSize)
		fs.Infof(o, "Reconstructed %s from odd+parity (degraded mode, streaming)", o.remote)

		out, err := newDecompressingReadCloser(reconstructor, degradedCompression)
		if err != nil {
			_ = reconstructor.Close()
			return nil, err
		}

		if rangeStart > 0 || rangeEnd >= 0 {
			return newRangeFilterReader(out, rangeStart, rangeEnd, o.Size()), nil
		}

		return out, nil
	}
	return nil, fmt.Errorf("cannot reconstruct: no data particle available")
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
	// Input validation
	if err := validateContext(ctx, "update"); err != nil {
		return err
	}
	if o == nil {
		return formatOperationError("update failed", "object cannot be nil", nil)
	}
	if err := validateRemote(o.remote, "update"); err != nil {
		return err
	}
	if err := validateObjectInfo(src, "update"); err != nil {
		return err
	}
	if in == nil {
		return formatOperationError("update failed", "input reader cannot be nil", nil)
	}

	return o.updateStreaming(ctx, in, src, options...)
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

	// Look up existing particle objects (with timeout to avoid hang on MinIO/S3)
	objCtx, cancel := context.WithTimeout(ctx, listBackendTimeout)
	defer cancel()
	evenObj, err := o.fs.even.NewObject(objCtx, o.remote)
	if err != nil {
		return fmt.Errorf("even particle not found: %w", err)
	}
	oddObj, err := o.fs.odd.NewObject(objCtx, o.remote)
	if err != nil {
		return fmt.Errorf("odd particle not found: %w", err)
	}

	var parityObj fs.Object
	parityObj, err = o.fs.parity.NewObject(objCtx, o.remote)
	if err != nil {
		parityObj = nil
	}
	parityName := o.remote

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
	compression, _ := ConfigToFooterCompression(o.fs.opt.Compression) // validated in NewFs
	if srcSize == 0 {
		mtime := effectiveModTime(ctx, o.fs, src, options)
		ft := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardEven)
		fb, _ := ft.MarshalBinary()
		evenR := bytes.NewReader(fb)
		ft2 := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardOdd)
		fb2, _ := ft2.MarshalBinary()
		oddR := bytes.NewReader(fb2)
		ft3 := FooterFromReconstructed(0, nil, nil, mtime, compression, ShardParity)
		fb3, _ := ft3.MarshalBinary()
		parityR := bytes.NewReader(fb3)
		evenInfo := createParticleInfo(o.fs, src, "even", FooterSize, false)
		err2 = evenObj.Update(ctx, evenR, evenInfo, options...)
		if err2 != nil {
			return fmt.Errorf("%s: failed to update even particle: %w", o.fs.even.Name(), err2)
		}
		uploadedParticles = append(uploadedParticles, evenObj)

		oddInfo := createParticleInfo(o.fs, src, "odd", FooterSize, false)
		err2 = oddObj.Update(ctx, oddR, oddInfo, options...)
		if err2 != nil {
			return fmt.Errorf("%s: failed to update odd particle: %w", o.fs.odd.Name(), err2)
		}
		uploadedParticles = append(uploadedParticles, oddObj)

		parityInfo := createParticleInfo(o.fs, src, "parity", FooterSize, false)
		parityInfo.remote = parityName
		if parityObj != nil {
			err2 = parityObj.Update(ctx, parityR, parityInfo, options...)
			if err2 != nil {
				return fmt.Errorf("%s: failed to update parity particle: %w", o.fs.parity.Name(), err2)
			}
			uploadedParticles = append(uploadedParticles, parityObj)
		} else {
			newParityObj, err := o.fs.parity.Put(ctx, parityR, parityInfo, options...)
			if err != nil {
				return fmt.Errorf("%s: failed to create parity particle: %w", o.fs.parity.Name(), err)
			}
			uploadedParticles = append(uploadedParticles, newParityObj)
		}

		return nil
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

	// Use errgroup for the Update goroutines; run the splitter in the calling goroutine
	// so the source reader is consumed on the same goroutine that called Update,
	// avoiding deadlocks with pipe/async readers (same pattern as Put in operations.go).
	g, gCtx := errgroup.WithContext(ctx)
	var uploadedMu sync.Mutex

	hasher := newHashingReader(in)
	splitInput := io.Reader(hasher)
	if o.fs.opt.Compression != "none" && o.fs.opt.Compression != "" {
		cr, err := newCompressingReader(hasher, o.fs.opt.Compression)
		if err != nil {
			return fmt.Errorf("compression: %w", err)
		}
		splitInput = cr
	}
	// Same producer goroutine + bufio as Put (operations.go) so the full stream is read
	producerPipeR, producerPipeW := io.Pipe()
	go func() {
		_, copyErr := io.Copy(producerPipeW, splitInput)
		_ = producerPipeW.CloseWithError(copyErr)
	}()
	splitInput = bufio.NewReaderSize(producerPipeR, streamProducerBufferSize)

	splitter := NewStreamSplitter(evenPipeW, oddPipeW, parityPipeW, streamReadChunkSize, isOddLengthCh)

	// Start Update goroutines first so they are reading from pipes when the splitter writes
	// Goroutine 1: Update even particle (reads from evenPipeR)
	g.Go(func() error {
		defer func() { _ = evenPipeR.Close() }()
		evenInfo := createParticleInfo(o.fs, src, "even", -1, isOddLength)
		err := evenObj.Update(gCtx, evenPipeR, evenInfo, options...)
		if err != nil {
			return fmt.Errorf("%s: failed to update even particle: %w", o.fs.even.Name(), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, evenObj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 2: Update odd particle (reads from oddPipeR)
	g.Go(func() error {
		defer func() { _ = oddPipeR.Close() }()
		oddInfo := createParticleInfo(o.fs, src, "odd", -1, isOddLength)
		err := oddObj.Update(gCtx, oddPipeR, oddInfo, options...)
		if err != nil {
			return fmt.Errorf("%s: failed to update odd particle: %w", o.fs.odd.Name(), err)
		}
		uploadedMu.Lock()
		uploadedParticles = append(uploadedParticles, oddObj)
		uploadedMu.Unlock()
		return nil
	})

	// Goroutine 3: Update or create parity particle (reads from parityPipeR)
	g.Go(func() error {
		defer func() { _ = parityPipeR.Close() }()

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

		parityInfo := createParticleInfo(o.fs, src, "parity", -1, newIsOddLength)
		parityInfo.remote = o.remote

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

	// Run splitter in this goroutine so the source reader is consumed here (same pattern as Put)
	errSplit := splitter.Split(splitInput)
	if errSplit != nil {
		_ = evenPipeW.CloseWithError(errSplit)
		_ = oddPipeW.CloseWithError(errSplit)
		_ = parityPipeW.CloseWithError(errSplit)
		gErr := g.Wait()
		if gErr != nil {
			return gErr
		}
		return errSplit
	}
	contentLength := hasher.ContentLength()
	md5Sum := hasher.MD5Sum()
	sha256Sum := hasher.SHA256Sum()
	mtime := effectiveModTime(gCtx, o.fs, src, options)
	for shard := 0; shard < 3; shard++ {
		ft := FooterFromReconstructed(contentLength, md5Sum[:], sha256Sum[:], mtime, compression, shard)
		fb, errMarshal := ft.MarshalBinary()
		if errMarshal != nil {
			_ = evenPipeW.Close()
			_ = oddPipeW.Close()
			_ = parityPipeW.Close()
			_ = g.Wait()
			return errMarshal
		}
		var w io.Writer
		switch shard {
		case ShardEven:
			w = evenPipeW
		case ShardOdd:
			w = oddPipeW
		case ShardParity:
			w = parityPipeW
		}
		if _, err := w.Write(fb); err != nil {
			_ = evenPipeW.Close()
			_ = oddPipeW.Close()
			_ = parityPipeW.Close()
			_ = g.Wait()
			return err
		}
	}
	_ = evenPipeW.Close()
	_ = oddPipeW.Close()
	_ = parityPipeW.Close()

	// Wait for all Update goroutines to complete
	if err2 = g.Wait(); err2 != nil {
		return err2
	}

	// Get written sizes from splitter for verification (payload + footer)
	totalEvenWritten := splitter.GetTotalEvenWritten() + FooterSize
	totalOddWritten := splitter.GetTotalOddWritten() + FooterSize

	// Verify sizes
	if err := verifyParticleSizes(ctx, o.fs, evenObj, oddObj, totalEvenWritten, totalOddWritten); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	return nil
}

// Remove removes the object
func (o *Object) Remove(ctx context.Context) error {
	// Input validation
	if err := validateContext(ctx, "remove"); err != nil {
		return err
	}
	if o == nil {
		return formatOperationError("remove failed", "object cannot be nil", nil)
	}
	if err := validateRemote(o.remote, "remove"); err != nil {
		return err
	}

	// Pre-flight check: Enforce strict RAID 3 delete policy
	// Fail immediately if any backend is unavailable to prevent partial deletes
	if err := o.fs.checkAllBackendsAvailable(ctx); err != nil {
		return formatOperationError("delete blocked in degraded mode (RAID 3 policy)", "", err)
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

	g.Go(func() error {
		obj, err := o.fs.parity.NewObject(gCtx, o.remote)
		if err == nil {
			return obj.Remove(gCtx)
		}
		return nil
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
// It returns the latest ModTime of all backends that have the directory
func (d *Directory) ModTime(ctx context.Context) time.Time {
	var latestTime time.Time
	backends := []fs.Fs{d.fs.even, d.fs.odd, d.fs.parity}

	// Get parent directory to list from
	parent := path.Dir(d.remote)
	if parent == "." {
		parent = ""
	}
	for _, backend := range backends {
		if backend == nil {
			continue
		}
		// List parent directory to find this directory entry
		entries, err := backend.List(ctx, parent)
		if err != nil {
			continue // Backend doesn't have parent directory
		}
		// Find the directory entry
		for _, entry := range entries {
			if dir, ok := entry.(fs.Directory); ok && dir.Remote() == d.remote {
				modTime := dir.ModTime(ctx)
				if latestTime.IsZero() || latestTime.Before(modTime) {
					latestTime = modTime
				}
				break
			}
		}
	}

	// If we didn't find any directory, return current time as fallback
	if latestTime.IsZero() {
		return time.Now()
	}
	return latestTime
}

// Metadata returns metadata for the directory
//
// It should return nil if there is no Metadata
func (d *Directory) Metadata(ctx context.Context) (fs.Metadata, error) {
	// Read metadata from all backends and merge them
	// This ensures consistency with ModTime() which reads from all backends
	parent := path.Dir(d.remote)
	if parent == "." {
		parent = ""
	}

	var mergedMetadata fs.Metadata
	backends := []fs.Fs{d.fs.even, d.fs.odd, d.fs.parity}

	for _, backend := range backends {
		if backend == nil {
			continue
		}
		entries, err := backend.List(ctx, parent)
		if err != nil {
			continue
		}
		// Find the directory entry
		for _, entry := range entries {
			if dir, ok := entry.(fs.Directory); ok && dir.Remote() == d.remote {
				if do, ok := dir.(fs.Metadataer); ok {
					meta, err := do.Metadata(ctx)
					if err == nil && meta != nil {
						if mergedMetadata == nil {
							mergedMetadata = make(fs.Metadata)
						}
						mergedMetadata.Merge(meta)
					}
				}
				break
			}
		}
	}

	// If we have metadata, ensure mtime matches ModTime() to avoid timing precision issues
	if mergedMetadata != nil {
		modTime := d.ModTime(ctx)
		mergedMetadata["mtime"] = modTime.Format(time.RFC3339Nano)
	}

	return mergedMetadata, nil
}

// SetMetadata sets metadata for the directory
//
// It should return fs.ErrorNotImplemented if it can't set metadata
func (d *Directory) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	// Check if directory exists before health check (union backend pattern)
	dirExists, err := d.fs.checkDirectoryExists(ctx, d.remote)
	if err != nil {
		return fmt.Errorf("failed to check directory existence: %w", err)
	}
	if !dirExists {
		return fs.ErrorDirNotFound
	}

	// Pre-flight check: Enforce strict RAID 3 write policy
	// SetMetadata is a metadata write operation
	// Consistent with DirSetModTime, Object.SetMetadata operations
	if err := d.fs.checkAllBackendsAvailable(ctx); err != nil {
		return fmt.Errorf("setmetadata blocked in degraded mode (RAID 3 policy): %w", err)
	}

	// Disable retries for strict RAID 3 write policy
	ctx = d.fs.disableRetriesForWrites(ctx)

	// Set metadata on all three backends that support it
	// Use errgroup to collect errors from all backends
	g, gCtx := errgroup.WithContext(ctx)

	parent := path.Dir(d.remote)
	if parent == "." {
		parent = ""
	}

	g.Go(func() error {
		entries, err := d.fs.even.List(gCtx, parent)
		if err != nil {
			return nil // Ignore if directory doesn't exist on this backend
		}
		for _, entry := range entries {
			if dir, ok := entry.(fs.Directory); ok && dir.Remote() == d.remote {
				if do, ok := dir.(fs.SetMetadataer); ok {
					err := do.SetMetadata(gCtx, metadata)
					if err != nil {
						return fmt.Errorf("%s: %w", d.fs.even.Name(), err)
					}
				}
				break
			}
		}
		return nil
	})

	g.Go(func() error {
		entries, err := d.fs.odd.List(gCtx, parent)
		if err != nil {
			return nil // Ignore if directory doesn't exist on this backend
		}
		for _, entry := range entries {
			if dir, ok := entry.(fs.Directory); ok && dir.Remote() == d.remote {
				if do, ok := dir.(fs.SetMetadataer); ok {
					err := do.SetMetadata(gCtx, metadata)
					if err != nil {
						return fmt.Errorf("%s: %w", d.fs.odd.Name(), err)
					}
				}
				break
			}
		}
		return nil
	})

	g.Go(func() error {
		entries, err := d.fs.parity.List(gCtx, parent)
		if err != nil {
			return nil // Ignore if directory doesn't exist on this backend
		}
		for _, entry := range entries {
			if dir, ok := entry.(fs.Directory); ok && dir.Remote() == d.remote {
				if do, ok := dir.(fs.SetMetadataer); ok {
					err := do.SetMetadata(gCtx, metadata)
					if err != nil {
						return fmt.Errorf("%s: %w", d.fs.parity.Name(), err)
					}
				}
				break
			}
		}
		return nil
	})

	return g.Wait()
}

// SetModTime sets the modification time of the directory
func (d *Directory) SetModTime(ctx context.Context, modTime time.Time) error {
	// Use Fs.DirSetModTime which already implements the logic
	if do := d.fs.Features().DirSetModTime; do != nil {
		return do(ctx, d.remote, modTime)
	}
	return fs.ErrorNotImplemented
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
