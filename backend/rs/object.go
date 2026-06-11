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
	"golang.org/x/sync/errgroup"
)

// errClosedStripeReader is returned from Read after Close on streaming logical readers.
var errClosedStripeReader = errors.New("rs: reader closed")

// Object is the logical file backed by Reed-Solomon shards.
type Object struct {
	fs           *Fs
	remote       string
	footer       *Footer
	primaryIndex int

	// Provisional list metadata (footer loaded lazily via ensureFooter).
	hasListSize    bool
	listSize       int64
	hasListModTime bool
	listModTime    time.Time
}

// Fs returns the parent rs filesystem.
func (o *Object) Fs() fs.Info { return o.fs }

func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the path of the logical object.
func (o *Object) Remote() string { return o.remote }

// ModTime returns the logical content modification time.
func (o *Object) ModTime(ctx context.Context) time.Time {
	if o.footer != nil {
		return time.Unix(0, o.footer.Mtime)
	}
	if o.hasListModTime {
		return o.listModTime
	}
	return time.Time{}
}

// Size returns the logical content length in bytes.
func (o *Object) Size() int64 {
	if o.footer != nil {
		return o.footer.ContentLength
	}
	if o.hasListSize {
		return o.listSize
	}
	return 0
}

// Storable reports whether the object can be stored (always true for rs).
func (o *Object) Storable() bool { return true }

// ensureFooter loads the EC footer from primaryIndex when list metadata is incomplete.
func (o *Object) ensureFooter(ctx context.Context) error {
	if o.footer != nil {
		return nil
	}
	if o.primaryIndex < 0 || o.primaryIndex >= len(o.fs.backends) {
		return fs.ErrorObjectNotFound
	}
	obj, err := o.fs.backends[o.primaryIndex].NewObject(ctx, o.remote)
	if err != nil {
		return err
	}
	ft, err := readFooterFromParticle(ctx, obj)
	if err != nil {
		return err
	}
	o.footer = ft
	return nil
}

// Hash returns MD5 or SHA256 of the logical content when supported.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	if err := o.ensureFooter(ctx); err != nil {
		return "", err
	}
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
	if err := o.ensureFooter(ctx); err != nil {
		return nil, err
	}
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

func stripeLogicalLen(k int, S, contentLength int64, stripeIdx int) int64 {
	return StripeLogicalLen(k, S, contentLength, stripeIdx)
}

func stripeLogicalBase(k int, S, contentLength int64, stripeIdx int) int64 {
	return StripeLogicalBase(k, S, contentLength, stripeIdx)
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
	if !probeDataShardsForOpen(ctx, o, k, N, S) {
		return nil, false, nil
	}

	start, endExclusive, ok := logicalSliceAfterOptions(cl, options...)
	if !ok || start >= endExclusive {
		return io.NopCloser(bytes.NewReader(nil)), true, nil
	}

	enc, err := reedsolomon.New(k, o.fs.opt.ParityShards)
	if err != nil {
		return nil, false, nil
	}

	r := &dataStripeStreamReader{
		ctx:          ctx,
		fs:           o.fs,
		remote:       o.remote,
		enc:          enc,
		k:            k,
		S:            S,
		N:            N,
		cl:           cl,
		start:        start,
		endExclusive: endExclusive,
	}
	return r, true, nil
}

// dataStripeStreamReader decodes logical content by joining stripe fragments from all k data
// shards only (no Reed–Solomon reconstruction). Fragment range reads do not verify the
// full-particle PayloadCRC32C; that matches prior buffered behavior for this path.
type dataStripeStreamReader struct {
	ctx          context.Context
	fs           *Fs
	remote       string
	enc          reedsolomon.Encoder
	k            int
	S            int64
	N            int
	cl           int64
	start        int64
	endExclusive int64

	t       int
	pending []byte
	closed  bool
}

func (r *dataStripeStreamReader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, errClosedStripeReader
	}
	if len(p) == 0 {
		return 0, nil
	}
	for n < len(p) {
		if len(r.pending) == 0 {
			if err := r.fill(); err != nil {
				if err == io.EOF {
					if n > 0 {
						return n, nil
					}
					return 0, io.EOF
				}
				return n, err
			}
		}
		copied := copy(p[n:], r.pending)
		r.pending = r.pending[copied:]
		n += copied
	}
	return n, nil
}

func (r *dataStripeStreamReader) fill() error {
	intS := int(r.S)
	for r.t < r.N {
		t := r.t
		logLen := stripeLogicalLen(r.k, r.S, r.cl, t)
		if logLen <= 0 {
			return io.EOF
		}
		base := stripeLogicalBase(r.k, r.S, r.cl, t)
		stripeEnd := base + logLen
		segStart := max(r.start, base)
		segEnd := min(r.endExclusive, stripeEnd)
		if segStart >= segEnd {
			r.t++
			continue
		}
		row := make([][]byte, r.k)
		buf := make([]byte, r.k*intS)
		for i := 0; i < r.k; i++ {
			row[i] = buf[i*intS : (i+1)*intS]
		}
		if err := readStripeFragmentsParallel(r.ctx, r.fs, r.remote, t, r.k, r.cl, r.S, logLen, row, true); err != nil {
			return err
		}
		var stripeOut bytes.Buffer
		joinN := r.k * intS
		if err := r.enc.Join(&stripeOut, row, joinN); err != nil {
			return err
		}
		joined := stripeOut.Bytes()
		if int64(len(joined)) < logLen {
			return fmt.Errorf("rs: stripe %d: join shorter than logical length", t)
		}
		joined = joined[:logLen]
		offInStripe := segStart - base
		segLen := segEnd - segStart
		seg := joined[offInStripe : offInStripe+segLen]
		r.pending = append(r.pending[:0], seg...)
		r.t++
		return nil
	}
	return io.EOF
}

func (r *dataStripeStreamReader) Close() error {
	r.closed = true
	return nil
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

// probeReconstructShardsPresent returns present[i]==true for each shard that has a readable
// particle matching the logical footer. Per-shard failures are ignored (same as sequential probe).
func probeReconstructShardsPresent(ctx context.Context, f *Fs, remote string, ref *Footer, k, m, N int, S int64) []bool {
	total := k + m
	present := make([]bool, total)
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < total; i++ {
		i := i
		g.Go(func() error {
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				return nil
			}
			if obj.Size() != ExpectedParticleSize(ref.ContentLength, i, k, m, int(S), true) {
				return nil
			}
			ft, err := readFooterFromParticle(gctx, obj)
			if err != nil {
				return nil
			}
			if !footerCompatibleForStripeRead(ref, ft, i) {
				return nil
			}
			present[i] = true
			return nil
		})
	}
	_ = g.Wait()
	return present
}

// probeDataShardsForOpen returns true iff all k data shards have a readable particle matching
// o.footer for stripe layout (same checks as sequential openViaDataShardsOnly probe).
func probeDataShardsForOpen(ctx context.Context, o *Object, k, N int, S int64) bool {
	type okOne struct{ ok bool }
	out := make([]okOne, k)
	g, gctx := errgroup.WithContext(ctx)
	for i := 0; i < k; i++ {
		i := i
		g.Go(func() error {
			obj, err := o.fs.backends[i].NewObject(gctx, o.remote)
			if err != nil {
				return nil
			}
			ft, err := readFooterFromParticle(gctx, obj)
			if err != nil {
				return nil
			}
			if !footerCompatibleForStripeRead(o.footer, ft, i) {
				return nil
			}
			if obj.Size() != ExpectedParticleSize(o.footer.ContentLength, i, k, o.fs.opt.ParityShards, int(S), true) {
				return nil
			}
			out[i].ok = true
			return nil
		})
	}
	_ = g.Wait()
	for i := 0; i < k; i++ {
		if !out[i].ok {
			return false
		}
	}
	return true
}

// readStripeFragmentsParallel reads one stripe's RS fragments into row (virtual-padding aware).
// Each non-nil row[i] must be at least S bytes; unread tail is zeroed for data shards.
func readStripeFragmentsParallel(ctx context.Context, f *Fs, remote string, stripeIdx, k int, contentLength, S, logLen int64, row [][]byte, dataPath bool) error {
	shardWord := "shard"
	if dataPath {
		shardWord = "data shard"
	}
	intS := int(S)
	g, gctx := errgroup.WithContext(ctx)
	for i := range row {
		if row[i] == nil {
			continue
		}
		i, dst := i, row[i]
		g.Go(func() error {
			var off int64
			var readLen int64
			if i < k {
				off = DataShardStripeOffset(i, k, intS, stripeIdx, contentLength)
				readLen = int64(DataShardFragLen(i, k, intS, int(logLen)))
				clear(dst)
			} else {
				off = int64(stripeIdx) * S
				readLen = S
			}
			if readLen == 0 {
				return nil
			}
			obj, err := f.backends[i].NewObject(gctx, remote)
			if err != nil {
				return fmt.Errorf("rs: %s %d: %w", shardWord, i, err)
			}
			rd, err := obj.Open(gctx, &fs.RangeOption{Start: off, End: off + readLen - 1})
			if err != nil {
				return fmt.Errorf("rs: %s %d: open fragment: %w", shardWord, i, err)
			}
			if _, err := io.ReadFull(rd, dst[:readLen]); err != nil {
				_ = rd.Close()
				return fmt.Errorf("rs: %s %d: read fragment: %w", shardWord, i, err)
			}
			if err := rd.Close(); err != nil {
				return fmt.Errorf("rs: %s %d: %w", shardWord, i, err)
			}
			return nil
		})
	}
	return g.Wait()
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
	cl := o.footer.ContentLength
	S := int64(o.footer.StripeSize)
	N := int(o.footer.NumStripes)

	// Stripe-wise streaming path: range-read fragments per stripe (no full-shard ReadAll).
	if S > 0 && N > 0 {
		start, endExclusive, ok := logicalSliceAfterOptions(cl, options...)
		if !ok {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		if start >= endExclusive {
			return io.NopCloser(bytes.NewReader(nil)), nil
		}
		present := probeReconstructShardsPresent(ctx, o.fs, o.remote, o.footer, k, m, N, S)
		available := 0
		for _, p := range present {
			if p {
				available++
			}
		}
		if available < k {
			return nil, fmt.Errorf("rs: not enough shards to reconstruct %q: have %d need %d", o.remote, available, k)
		}
		enc, err := reedsolomon.New(k, m)
		if err != nil {
			return nil, err
		}
		intS := int(S)
		bufs := make([]byte, (k+m)*intS)
		row := make([][]byte, k+m)
		for i := 0; i < k+m; i++ {
			if present[i] {
				row[i] = bufs[i*intS : (i+1)*intS]
			}
		}
		r := &fullReconstructStripeReader{
			ctx:          ctx,
			fs:           o.fs,
			remote:       o.remote,
			enc:          enc,
			k:            k,
			m:            m,
			S:            S,
			intS:         intS,
			N:            N,
			cl:           cl,
			start:        start,
			endExclusive: endExclusive,
			row:          row,
			present:      present,
		}
		return r, nil
	}

	// Non-stripe layout: buffered path with full-particle reads and CRC verification.
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
	out, err := ReconstructDataFromShards(shards, k, m, cl, o.footer.StripeSize, o.footer.NumStripes)
	if err != nil {
		return nil, err
	}
	out = applyReadOptions(out, options...)
	return io.NopCloser(bytes.NewReader(out)), nil
}

// fullReconstructStripeReader decodes logical content stripe-by-stripe using RS reconstruction
// when some shards are missing. Fragment reads do not verify full-particle PayloadCRC32C.
type fullReconstructStripeReader struct {
	ctx          context.Context
	fs           *Fs
	remote       string
	enc          reedsolomon.Encoder
	k, m         int
	S            int64
	intS         int
	N            int
	cl           int64
	start        int64
	endExclusive int64

	t       int
	pending []byte
	closed  bool

	row     [][]byte
	present []bool
}

func (r *fullReconstructStripeReader) Read(p []byte) (n int, err error) {
	if r.closed {
		return 0, errClosedStripeReader
	}
	if len(p) == 0 {
		return 0, nil
	}
	for n < len(p) {
		if len(r.pending) == 0 {
			if err := r.fill(); err != nil {
				if err == io.EOF {
					if n > 0 {
						return n, nil
					}
					return 0, io.EOF
				}
				return n, err
			}
		}
		copied := copy(p[n:], r.pending)
		r.pending = r.pending[copied:]
		n += copied
	}
	return n, nil
}

func (r *fullReconstructStripeReader) fill() error {
	for {
		if r.t >= r.N {
			return io.EOF
		}
		logicalLen := stripeLogicalLen(r.k, r.S, r.cl, r.t)
		if logicalLen <= 0 {
			return io.EOF
		}
		base := stripeLogicalBase(r.k, r.S, r.cl, r.t)
		stripeEnd := base + logicalLen
		segStart := max(r.start, base)
		segEnd := min(r.endExclusive, stripeEnd)
		if segStart >= segEnd {
			r.t++
			continue
		}
		t := r.t
		for i := 0; i < r.k+r.m; i++ {
			if !r.present[i] {
				r.row[i] = nil
			}
		}
		if err := readStripeFragmentsParallel(r.ctx, r.fs, r.remote, t, r.k, r.cl, r.S, logicalLen, r.row, false); err != nil {
			return err
		}
		joined, err := reconstructStripeJoined(r.enc, r.row, r.k, r.m, r.intS, logicalLen, t)
		if err != nil {
			return err
		}
		offInStripe := segStart - base
		segLen := segEnd - segStart
		seg := joined[offInStripe : offInStripe+segLen]
		r.pending = append(r.pending[:0], seg...)
		r.t++
		return nil
	}
}

func (r *fullReconstructStripeReader) Close() error {
	r.closed = true
	return nil
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
	if err := o.ensureFooter(ctx); err != nil {
		return err
	}
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
	o.footer.Mtime = t.UnixNano()
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
	newFt.Mtime = t.UnixNano()
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

// reconstructStripeJoined runs Reed-Solomon reconstruction (if needed) and join for one stripe.
// row must have length k+m with nil entries for missing shards; non-nil entries must each be length S.
func reconstructStripeJoined(enc reedsolomon.Encoder, row [][]byte, k, m, S int, logicalLen int64, stripeIdx int) ([]byte, error) {
	available := 0
	for _, r := range row {
		if r != nil {
			available++
		}
	}
	if available < k {
		return nil, fmt.Errorf("rs: stripe %d: insufficient shards (have %d need %d)", stripeIdx, available, k)
	}
	if err := enc.Reconstruct(row); err != nil {
		return nil, fmt.Errorf("rs: stripe %d: %w", stripeIdx, err)
	}
	var stripeBuf bytes.Buffer
	if err := enc.Join(&stripeBuf, row[:k], k*S); err != nil {
		return nil, err
	}
	b := stripeBuf.Bytes()
	if int64(len(b)) < logicalLen {
		return nil, fmt.Errorf("rs: stripe %d: join shorter than logical length", stripeIdx)
	}
	return b[:logicalLen], nil
}

// ReconstructDataFromShards decodes stripe-wise RS particles (footer v1) into logical content.
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
		if s == nil {
			continue
		}
		want := int(ShardPayloadLen(contentLength, i, k, m, S))
		if len(s) != want {
			return nil, fmt.Errorf("rs: shard %d payload length %d, want %d", i, len(s), want)
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
				frag := shardStripeFragmentFromPayload(shards[i], i, k, S, t, contentLength, logicalLen)
				if frag == nil {
					return nil, fmt.Errorf("rs: shard %d stripe %d: fragment out of range", i, t)
				}
				buf := make([]byte, S)
				copy(buf, frag)
				row[i] = buf
			}
		}
		b, err := reconstructStripeJoined(enc, row, k, m, S, logicalLen, t)
		if err != nil {
			return nil, err
		}
		if _, err := out.Write(b); err != nil {
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
