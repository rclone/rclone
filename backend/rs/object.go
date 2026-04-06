package rs

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"golang.org/x/sync/errgroup"
)

// Object is the logical file backed by Reed-Solomon shards.
type Object struct {
	fs           *Fs
	remote       string
	footer       *Footer
	primaryIndex int
}

// Fs returns the parent rs filesystem.
func (o *Object) Fs() fs.Info { return o.fs }

func (o *Object) String() string { return o.remote }

// Remote returns the path of the logical object.
func (o *Object) Remote() string { return o.remote }

// ModTime returns the logical content modification time.
func (o *Object) ModTime(ctx context.Context) time.Time { return time.Unix(o.footer.Mtime, 0) }

// Size returns the logical content length in bytes.
func (o *Object) Size() int64 { return o.footer.ContentLength }

// Storable reports whether the object can be stored (always true for rs).
func (o *Object) Storable() bool { return true }

// Hash returns MD5 or SHA256 of the logical content when supported.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	switch ty {
	case hash.MD5:
		return hex.EncodeToString(o.footer.MD5[:]), nil
	case hash.SHA256:
		return hex.EncodeToString(o.footer.SHA256[:]), nil
	default:
		return "", hash.ErrUnsupported
	}
}

// Open reads logical content, using data shards when possible or full reconstruction.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	cl := o.footer.ContentLength
	if cl == 0 {
		b := applyReadOptions([]byte{}, options...)
		if b == nil {
			b = []byte{}
		}
		return io.NopCloser(bytes.NewReader(b)), nil
	}

	if o.footer.StripeSize > 0 {
		rc, used, err := o.openViaDataShardsOnly(ctx, int64(o.footer.StripeSize), cl, options)
		if used {
			return rc, err
		}
	}
	return o.openFullReconstruct(ctx, options...)
}

// openViaDataShardsOnly returns (reader, true, err) when all k data shards are present and
// the logical range is served by sequential split layout (klauspost Split / Join).
// Returns (nil, false, nil) to fall back to full Reed–Solomon reconstruction.
func (o *Object) openViaDataShardsOnly(ctx context.Context, stripeSize int64, contentLength int64, options []fs.OpenOption) (io.ReadCloser, bool, error) {
	k := o.fs.opt.DataShards
	if stripeSize <= 0 {
		return nil, false, nil
	}
	for i := 0; i < k; i++ {
		obj, err := o.fs.backends[i].NewObject(ctx, o.remote)
		if err != nil {
			return nil, false, nil
		}
		ft, err := readFooterFromParticle(ctx, obj)
		if err != nil {
			return nil, false, nil
		}
		if !footerCompatibleForSequentialRead(o.footer, ft, i) {
			return nil, false, nil
		}
	}

	start, endExclusive, ok := logicalSliceAfterOptions(contentLength, options...)
	if !ok || start >= endExclusive {
		return io.NopCloser(bytes.NewReader(nil)), true, nil
	}

	var buf bytes.Buffer
	pos := start
	for pos < endExclusive {
		shardIdx := pos / stripeSize
		if int(shardIdx) >= k {
			return nil, false, nil
		}
		localOff := pos % stripeSize
		shardLogicalEnd := (shardIdx + 1) * stripeSize
		if shardLogicalEnd > contentLength {
			shardLogicalEnd = contentLength
		}
		readEnd := endExclusive
		if readEnd > shardLogicalEnd {
			readEnd = shardLogicalEnd
		}
		n := readEnd - pos
		if n <= 0 {
			return nil, false, nil
		}
		localReadEnd := localOff + n
		if localReadEnd > stripeSize {
			return nil, false, nil
		}
		obj, err := o.fs.backends[shardIdx].NewObject(ctx, o.remote)
		if err != nil {
			return nil, false, nil
		}
		rd, err := obj.Open(ctx, &fs.RangeOption{Start: localOff, End: localReadEnd - 1})
		if err != nil {
			return nil, false, nil
		}
		_, err = io.Copy(&buf, rd)
		_ = rd.Close()
		if err != nil {
			return nil, false, nil
		}
		pos += n
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), true, nil
}

func footerCompatibleForSequentialRead(ref *Footer, ft *Footer, shardIndex int) bool {
	if ft == nil || ref == nil {
		return false
	}
	if ft.ContentLength != ref.ContentLength {
		return false
	}
	if ft.DataShards != ref.DataShards || ft.ParityShards != ref.ParityShards {
		return false
	}
	if ft.StripeSize != ref.StripeSize {
		return false
	}
	if int(ft.CurrentShard) != shardIndex {
		return false
	}
	return ft.Algorithm == ref.Algorithm
}

// logicalSliceAfterOptions computes [start, endExclusive) in the logical object, matching the
// semantics of applyReadOptions (Seek then Range on the remaining buffer).
func logicalSliceAfterOptions(contentLength int64, options ...fs.OpenOption) (start, endExclusive int64, ok bool) {
	if contentLength <= 0 {
		return 0, 0, false
	}
	start, endExclusive = 0, contentLength
	for _, opt := range options {
		switch x := opt.(type) {
		case *fs.SeekOption:
			if x.Offset < 0 || x.Offset >= endExclusive-start {
				return 0, 0, false
			}
			start += x.Offset
		case *fs.RangeOption:
			segLen := endExclusive - start
			off, lim := x.Decode(segLen)
			if off < 0 || off > segLen {
				return 0, 0, false
			}
			start += off
			if lim < 0 {
				endExclusive = start + (segLen - off)
			} else {
				end := start + lim
				maxEnd := start + (segLen - off)
				if end > maxEnd {
					end = maxEnd
				}
				if end < start {
					return 0, 0, false
				}
				endExclusive = end
			}
		default:
			// ignore unknown options (same as applyReadOptions)
		}
	}
	if start >= endExclusive {
		return 0, 0, false
	}
	return start, endExclusive, true
}

func (o *Object) openFullReconstruct(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	k := o.fs.opt.DataShards
	m := o.fs.opt.ParityShards
	shards := make([][]byte, k+m)
	for i, b := range o.fs.backends {
		obj, err := b.NewObject(ctx, o.remote)
		if err != nil {
			continue
		}
		r, err := obj.Open(ctx)
		if err != nil {
			continue
		}
		data, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			continue
		}
		payload, _, err := ExtractParticlePayload(data, i)
		if err != nil {
			continue
		}
		shards[i] = payload
	}
	available := 0
	for _, s := range shards {
		if s != nil {
			available++
		}
	}
	if available < k {
		return nil, fmt.Errorf("rs: not enough shards to reconstruct %q: have %d need %d", o.remote, available, k)
	}
	out, err := ReconstructDataFromShards(shards, k, m, o.footer.ContentLength)
	if err != nil {
		return nil, err
	}
	out = applyReadOptions(out, options...)
	return io.NopCloser(bytes.NewReader(out)), nil
}

// Update replaces the logical object content via Put and refreshes metadata on o.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Use this object's remote; ignore src.Remote() (fstest passes a deliberate wrong name).
	info := object.NewStaticObjectInfo(o.remote, src.ModTime(ctx), src.Size(), src.Storable(), nil, o.fs)
	if _, err := o.fs.Put(ctx, in, info, options...); err != nil {
		return err
	}
	// Refresh metadata on the receiver (Put replaces shards; old footer would be stale).
	newObj, err := o.fs.NewObject(ctx, o.remote)
	if err != nil {
		return err
	}
	ro := newObj.(*Object)
	o.footer = ro.footer
	o.primaryIndex = ro.primaryIndex
	return nil
}

// Remove deletes the object from all shards that hold it.
func (o *Object) Remove(ctx context.Context) error {
	var errs int
	for _, b := range o.fs.backends {
		obj, err := b.NewObject(ctx, o.remote)
		if err != nil {
			continue
		}
		if err := obj.Remove(ctx); err != nil {
			errs++
		}
	}
	if errs > 0 {
		return fmt.Errorf("rs: failed removing one or more shards for %q", o.remote)
	}
	return nil
}

// SetModTime updates the EC footer mtime on every shard; payload bytes are unchanged.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	g, gCtx := errgroup.WithContext(ctx)
	for i := range o.fs.backends {
		i := i
		b := o.fs.backends[i]
		g.Go(func() error {
			return o.updateShardFooterMtime(gCtx, b, i, t)
		})
	}
	if err := g.Wait(); err != nil {
		return err
	}
	o.footer.Mtime = t.Unix()
	return nil
}

func (o *Object) updateShardFooterMtime(ctx context.Context, b fs.Fs, shardIdx int, t time.Time) error {
	obj, err := b.NewObject(ctx, o.remote)
	if err != nil {
		return fmt.Errorf("rs: shard %d: %w", shardIdx, err)
	}
	size := obj.Size()
	if size < FooterSize {
		return fmt.Errorf("rs: shard %d: blob too small for footer (%d bytes)", shardIdx, size)
	}
	rd, err := obj.Open(ctx, &fs.RangeOption{Start: size - FooterSize, End: size - 1})
	if err != nil {
		return fmt.Errorf("rs: shard %d: read footer: %w", shardIdx, err)
	}
	footerBuf := make([]byte, FooterSize)
	if _, err := io.ReadFull(rd, footerBuf); err != nil {
		_ = rd.Close()
		return fmt.Errorf("rs: shard %d: read footer: %w", shardIdx, err)
	}
	_ = rd.Close()
	ft, err := ParseFooter(footerBuf)
	if err != nil {
		return fmt.Errorf("rs: shard %d: %w", shardIdx, err)
	}
	newFt := *ft
	newFt.Mtime = t.Unix()
	fb, err := newFt.MarshalBinary()
	if err != nil {
		return fmt.Errorf("rs: shard %d: marshal footer: %w", shardIdx, err)
	}

	var combined io.Reader
	if size == FooterSize {
		combined = bytes.NewReader(fb)
	} else {
		pldRd, err := obj.Open(ctx, &fs.RangeOption{Start: 0, End: size - FooterSize - 1})
		if err != nil {
			return fmt.Errorf("rs: shard %d: read payload: %w", shardIdx, err)
		}
		payload, err := io.ReadAll(pldRd)
		_ = pldRd.Close()
		if err != nil {
			return fmt.Errorf("rs: shard %d: read payload: %w", shardIdx, err)
		}
		combined = io.MultiReader(bytes.NewReader(payload), bytes.NewReader(fb))
	}
	info := object.NewStaticObjectInfo(o.remote, t, size, true, nil, nil)
	if err := obj.Update(ctx, combined, info); err != nil {
		return fmt.Errorf("rs: shard %d: update footer mtime: %w", shardIdx, err)
	}
	return nil
}

func readFooterFromParticle(ctx context.Context, obj fs.Object) (*Footer, error) {
	size := obj.Size()
	if size < FooterSize {
		return nil, fmt.Errorf("rs: particle too small for footer: %d", size)
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

// ExtractParticlePayload parses the trailing EC footer, verifies PayloadCRC32C, and returns
// the payload bytes and footer. If wantShardIndex >= 0, CurrentShard must match.
func ExtractParticlePayload(particle []byte, wantShardIndex int) ([]byte, *Footer, error) {
	if len(particle) < FooterSize {
		return nil, nil, fmt.Errorf("rs: particle too small for footer: %d", len(particle))
	}
	payload := particle[:len(particle)-FooterSize]
	ft, err := ParseFooter(particle[len(particle)-FooterSize:])
	if err != nil {
		return nil, nil, err
	}
	if wantShardIndex >= 0 && int(ft.CurrentShard) != wantShardIndex {
		return nil, nil, fmt.Errorf("rs: shard index mismatch: expected=%d got=%d", wantShardIndex, ft.CurrentShard)
	}
	if crc32cChecksum(payload) != ft.PayloadCRC32C {
		return nil, nil, fmt.Errorf("rs: shard crc mismatch for shard %d", ft.CurrentShard)
	}
	return payload, ft, nil
}

// ReconstructDataFromShards runs Reed-Solomon reconstruct + join to the original content length.
func ReconstructDataFromShards(shards [][]byte, dataShards, parityShards int, contentLength int64) ([]byte, error) {
	available := 0
	for _, s := range shards {
		if s != nil {
			available++
		}
	}
	if available < dataShards {
		return nil, fmt.Errorf("rs: insufficient shards for reconstruction: have %d need %d", available, dataShards)
	}
	enc, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil, err
	}
	if err := enc.Reconstruct(shards); err != nil {
		return nil, err
	}
	if contentLength > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("rs: object too large to join in memory")
	}
	var buf bytes.Buffer
	if err := enc.Join(&buf, shards, int(contentLength)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func applyReadOptions(data []byte, options ...fs.OpenOption) []byte {
	out := data
	for _, opt := range options {
		switch x := opt.(type) {
		case *fs.SeekOption:
			if x.Offset >= int64(len(out)) {
				return nil
			}
			if x.Offset > 0 {
				out = out[x.Offset:]
			}
		case *fs.RangeOption:
			segLen := int64(len(out))
			off, lim := x.Decode(segLen)
			if off < 0 || off > segLen {
				return nil
			}
			if lim < 0 {
				out = out[off:]
			} else {
				end := off + lim
				if end > segLen {
					end = segLen
				}
				if end < off {
					return nil
				}
				out = out[off:end]
			}
		}
	}
	return out
}
