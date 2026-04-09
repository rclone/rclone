package rs

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/klauspost/reedsolomon"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
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

	if o.footer.StripeSize > 0 && o.footer.NumStripes > 0 {
		rc, used, err := o.openViaDataShardsOnly(ctx, options)
		if used {
			return rc, err
		}
	}
	return o.openFullReconstruct(ctx, options...)
}

// stripeLogicalLen returns how many logical bytes are in stripe stripeIdx (0-based).
func stripeLogicalLen(k int, S, contentLength int64, stripeIdx int) int64 {
	kS := int64(k) * S
	var pos int64
	for s := 0; s < stripeIdx; s++ {
		chunk := min(kS, contentLength-pos)
		if chunk <= 0 {
			return 0
		}
		pos += chunk
	}
	return min(kS, contentLength-pos)
}

// stripeLogicalBase returns the logical byte offset where stripe stripeIdx starts.
func stripeLogicalBase(k int, S, contentLength int64, stripeIdx int) int64 {
	var base int64
	for s := 0; s < stripeIdx; s++ {
		base += stripeLogicalLen(k, S, contentLength, s)
	}
	return base
}

// openViaDataShardsOnly returns (reader, true, err) when all k data shards are present and
// the object uses stripe-wise layout (NumStripes, StripeSize).
// Returns (nil, false, nil) to fall back to full Reed–Solomon reconstruction.
func (o *Object) openViaDataShardsOnly(ctx context.Context, options []fs.OpenOption) (io.ReadCloser, bool, error) {
	k := o.fs.opt.DataShards
	S := int64(o.footer.StripeSize)
	N := int(o.footer.NumStripes)
	cl := o.footer.ContentLength
	if S <= 0 || N <= 0 {
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
		if !footerCompatibleForStripeRead(o.footer, ft, i) {
			return nil, false, nil
		}
		if obj.Size() != int64(N)*S+FooterSize {
			return nil, false, nil
		}
	}

	start, endExclusive, ok := logicalSliceAfterOptions(cl, options...)
	if !ok || start >= endExclusive {
		return io.NopCloser(bytes.NewReader(nil)), true, nil
	}

	enc, err := reedsolomon.New(k, o.fs.opt.ParityShards)
	if err != nil {
		return nil, false, nil
	}

	var buf bytes.Buffer
	for t := 0; t < N; t++ {
		base := stripeLogicalBase(k, S, cl, t)
		logLen := stripeLogicalLen(k, S, cl, t)
		if logLen <= 0 {
			continue
		}
		stripeEnd := base + logLen
		segStart := max(start, base)
		segEnd := min(endExclusive, stripeEnd)
		if segStart >= segEnd {
			continue
		}
		row := make([][]byte, k)
		off := int64(t) * S
		for i := 0; i < k; i++ {
			obj, err := o.fs.backends[i].NewObject(ctx, o.remote)
			if err != nil {
				return nil, false, nil
			}
			rd, err := obj.Open(ctx, &fs.RangeOption{Start: off, End: off + S - 1})
			if err != nil {
				return nil, false, nil
			}
			frag := make([]byte, S)
			if _, err := io.ReadFull(rd, frag); err != nil {
				_ = rd.Close()
				return nil, false, nil
			}
			_ = rd.Close()
			row[i] = frag
		}
		var stripeOut bytes.Buffer
		joinN := k * int(S)
		if err := enc.Join(&stripeOut, row, joinN); err != nil {
			return nil, false, nil
		}
		joined := stripeOut.Bytes()
		if int64(len(joined)) < logLen {
			return nil, false, nil
		}
		joined = joined[:logLen]
		offInStripe := segStart - base
		slice := joined[offInStripe : offInStripe+(segEnd-segStart)]
		if _, err := buf.Write(slice); err != nil {
			return nil, false, err
		}
	}
	return io.NopCloser(bytes.NewReader(buf.Bytes())), true, nil
}

func footerCompatibleForStripeRead(ref *Footer, ft *Footer, shardIndex int) bool {
	if ft == nil || ref == nil {
		return false
	}
	if ft.ContentLength != ref.ContentLength {
		return false
	}
	if ft.DataShards != ref.DataShards || ft.ParityShards != ref.ParityShards {
		return false
	}
	if ft.StripeSize != ref.StripeSize || ft.NumStripes != ref.NumStripes {
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
	out, err := ReconstructDataFromShards(shards, k, m, o.footer.ContentLength, o.footer.StripeSize, o.footer.NumStripes)
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

// Remove deletes the object from quorum shards in two phases.
func (o *Object) Remove(ctx context.Context) error {
	targets := make([]int, 0, len(o.fs.backends))
	for i, b := range o.fs.backends {
		obj, err := b.NewObject(ctx, o.remote)
		if err != nil {
			continue
		}
		_ = obj
		targets = append(targets, i)
	}
	if len(targets) == 0 {
		return fs.ErrorObjectNotFound
	}
	result := o.fs.runTwoPhaseQuorumOp(ctx, "remove", o.remote, targets, func(opCtx context.Context, shard int) error {
		obj, err := o.fs.backends[shard].NewObject(opCtx, o.remote)
		if err != nil {
			return nil
		}
		return obj.Remove(opCtx)
	})
	if result.Successes < o.fs.writeQuorum() {
		return fmt.Errorf("rs: remove quorum not met for %q: successes=%d required=%d", o.remote, result.Successes, o.fs.writeQuorum())
	}
	return nil
}

// SetModTime updates EC footer mtimes with quorum semantics.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	targets := make([]int, 0, len(o.fs.backends))
	for i := range o.fs.backends {
		targets = append(targets, i)
	}
	result := o.fs.runTwoPhaseQuorumOp(ctx, "setmodtime", o.remote, targets, func(opCtx context.Context, shard int) error {
		return o.updateShardFooterMtime(opCtx, o.fs.backends[shard], shard, t)
	})
	if result.Successes < o.fs.writeQuorum() {
		return fmt.Errorf("rs: setmodtime quorum not met for %q: successes=%d required=%d", o.remote, result.Successes, o.fs.writeQuorum())
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

// ReconstructDataFromShards decodes stripe-wise RS particles (footer v3) into logical content.
func ReconstructDataFromShards(shards [][]byte, dataShards, parityShards int, contentLength int64, stripeSize, numStripes uint32) ([]byte, error) {
	k, m := dataShards, parityShards
	S := int(stripeSize)
	N := int(numStripes)
	if contentLength == 0 {
		return []byte{}, nil
	}
	if S <= 0 || N <= 0 {
		return nil, errors.New("rs: invalid stripe metadata (StripeSize/NumStripes)")
	}
	for i, s := range shards {
		if s != nil && len(s) != N*S {
			return nil, fmt.Errorf("rs: shard %d payload length %d, want %d", i, len(s), N*S)
		}
	}
	enc, err := reedsolomon.New(k, m)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	var logicalPos int64
	for t := 0; t < N; t++ {
		logicalLen := stripeLogicalLen(k, int64(S), contentLength, t)
		if logicalLen <= 0 {
			break
		}
		row := make([][]byte, k+m)
		for i := 0; i < k+m; i++ {
			if shards[i] != nil {
				row[i] = shards[i][t*S : (t+1)*S]
			}
		}
		available := 0
		for _, r := range row {
			if r != nil {
				available++
			}
		}
		if available < k {
			return nil, fmt.Errorf("rs: stripe %d: insufficient shards (have %d need %d)", t, available, k)
		}
		if err := enc.Reconstruct(row); err != nil {
			return nil, fmt.Errorf("rs: stripe %d: %w", t, err)
		}
		var stripeBuf bytes.Buffer
		if err := enc.Join(&stripeBuf, row[:k], k*S); err != nil {
			return nil, err
		}
		b := stripeBuf.Bytes()
		if int64(len(b)) < logicalLen {
			return nil, fmt.Errorf("rs: stripe %d: join shorter than logical length", t)
		}
		if _, err := out.Write(b[:logicalLen]); err != nil {
			return nil, err
		}
		logicalPos += logicalLen
	}
	if logicalPos != contentLength {
		return nil, fmt.Errorf("rs: decoded length %d want %d", logicalPos, contentLength)
	}
	return out.Bytes(), nil
}

// ReconstructDataFromShardsFlat performs a single whole-buffer Split/Join decode (legacy layout).
// It is used by rsverify --raw where shards are raw RS pieces without stripe metadata.
func ReconstructDataFromShardsFlat(shards [][]byte, dataShards, parityShards int, contentLength int64) ([]byte, error) {
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
	cp := make([][]byte, len(shards))
	copy(cp, shards)
	if err := enc.Reconstruct(cp); err != nil {
		return nil, err
	}
	if contentLength > int64(^uint(0)>>1) {
		return nil, fmt.Errorf("rs: object too large to join in memory")
	}
	var buf bytes.Buffer
	if err := enc.Join(&buf, cp[:dataShards], int(contentLength)); err != nil {
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
