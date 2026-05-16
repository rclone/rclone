package rs

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"math/rand"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/memory"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func TestParseRemotes(t *testing.T) {
	got := parseRemotes(" a:,b:bucket , c:path ,,")
	require.Equal(t, []string{"a:", "b:bucket", "c:path"}, got)
}

func TestValidateOptions(t *testing.T) {
	opt := &Options{
		Remotes:            "a:,b:,c:",
		DataShards:         2,
		ParityShards:       1,
	}
	require.NoError(t, validateOptions(opt))
	require.Equal(t, DefaultStripeFragmentSize, opt.StripeFragmentSize)

	opt2 := &Options{Remotes: "a:,b:,c:", DataShards: 2, ParityShards: 1, StripeFragmentSize: 32}
	require.Error(t, validateOptions(opt2))

	opt3 := &Options{Remotes: "a:,b:,c:,d:", DataShards: 2, ParityShards: 2}
	require.Error(t, validateOptions(opt3))
	require.Contains(t, validateOptions(opt3).Error(), "k > m")

	opt4 := &Options{Remotes: "a:,b:,c:", DataShards: 1, ParityShards: 2}
	require.Error(t, validateOptions(opt4))
	require.Contains(t, validateOptions(opt4).Error(), "k > m")
}

func TestBuildRSShardsToWriters(t *testing.T) {
	ctx := context.Background()
	data := bytes.Repeat([]byte("abc123"), 100)
	src := object.NewStaticObjectInfo("x.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)

	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}

	res, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), res.ContentLength)
	require.Equal(t, uint32(DefaultStripeFragmentSize), res.StripeSize)
	require.Equal(t, 1, NumStripesForContent(res.ContentLength, 2, int(res.StripeSize)))

	for i := range writers {
		raw := writers[i].Bytes()
		require.GreaterOrEqual(t, len(raw), FooterSize)
		payload := raw[:len(raw)-FooterSize]
		tail := raw[len(raw)-FooterSize:]
		ft, err := ParseFooter(tail)
		require.NoError(t, err)
		require.Equal(t, FooterVersion, int(binary.LittleEndian.Uint16(tail[9:11])))
		require.Equal(t, uint8(2), ft.DataShards)
		require.Equal(t, uint8(2), ft.ParityShards)
		require.Equal(t, uint8(i), ft.CurrentShard)
		require.Equal(t, AlgorithmRS, ft.Algorithm)
		require.Equal(t, uint32(1), ft.NumStripes)
		require.Equal(t, res.StripeSize, ft.StripeSize)
		require.Equal(t, crc32cChecksum(payload), ft.PayloadCRC32C)
	}
}

func TestBuildRSShardsDeclaredSizeMismatch(t *testing.T) {
	ctx := context.Background()
	data := []byte("hello")
	tooLarge := object.NewStaticObjectInfo("x.bin", time.Unix(1700000000, 0), 100, true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), tooLarge, 2, 2, 0, ios, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "incorrect upload size")
	// Short stream vs large declared size: stripes are encoded before final EOF reveals truncation (chunker-style).

	writers2 := make([]*bytes.Buffer, 4)
	ios2 := make([]io.Writer, 4)
	for i := range writers2 {
		writers2[i] = &bytes.Buffer{}
		ios2[i] = writers2[i]
	}
	tooSmall := object.NewStaticObjectInfo("y.bin", time.Unix(1700000000, 0), 2, true, nil, nil)
	_, err = BuildRSShardsToWriters(ctx, bytes.NewReader(data), tooSmall, 2, 2, 0, ios2, true)
	require.Error(t, err)
	require.Contains(t, err.Error(), "incorrect upload size")
	for i := range writers2 {
		require.Zero(t, writers2[i].Len(), "oversized stream vs declared should fail before write")
	}
}

func TestBuildRSShardsUnknownSizeSkipsDeclaredCheck(t *testing.T) {
	ctx := context.Background()
	data := bytes.Repeat([]byte("z"), 50)
	src := object.NewStaticObjectInfo("u.bin", time.Unix(1700000000, 0), -1, true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	res, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), res.ContentLength)
}

func TestShardParticleFileSize(t *testing.T) {
	const k, stripeS = 2, 32
	cl := int64(100)
	// NumStripes = ceil(100 / (2*32)) = 2; payload = 2*32 = 64
	require.Equal(t, int64(64), ShardPayloadByteLength(cl, k, stripeS))
	require.Equal(t, int64(64+FooterSize), ShardParticleFileSize(cl, k, stripeS, true))
	require.Equal(t, int64(64), ShardParticleFileSize(cl, k, stripeS, false))
}

// strictMaxReadReader fails if a single Read is asked for more than maxBytes (streaming encode uses at most k*S per Read cycle from the underlying reader when ReadFull fills a k*S buffer).
type strictMaxReadReader struct {
	data    []byte
	pos     int
	maxRead int
}

func (s *strictMaxReadReader) Read(p []byte) (int, error) {
	if len(p) > s.maxRead {
		return 0, errors.New("read exceeds max chunk")
	}
	if s.pos >= len(s.data) {
		return 0, io.EOF
	}
	n := copy(p, s.data[s.pos:])
	s.pos += n
	return n, nil
}

func TestBuildRSShardsStreamingReadBound(t *testing.T) {
	ctx := context.Background()
	const k, m, stripeS = 2, 2, 64
	logicalStripe := k * stripeS
	data := bytes.Repeat([]byte("q"), logicalStripe*3+17)
	src := object.NewStaticObjectInfo("bound.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, k+m)
	ios := make([]io.Writer, k+m)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	in := &strictMaxReadReader{data: data, maxRead: logicalStripe}
	_, err := BuildRSShardsToWriters(ctx, in, src, k, m, stripeS, ios, true)
	require.NoError(t, err)
}

func TestApplyReadOptions(t *testing.T) {
	base := []byte("0123456789")
	require.Equal(t, []byte("3456789"), applyReadOptions(base, &fs.SeekOption{Offset: 3}))
	require.Equal(t, []byte("234"), applyReadOptions(base, &fs.RangeOption{Start: 2, End: 4}))
	require.Nil(t, applyReadOptions(base, &fs.RangeOption{Start: 20, End: 30}))
}

func TestLogicalSliceAfterOptionsMatchesApplyRead(t *testing.T) {
	base := []byte("0123456789")
	cl := int64(len(base))
	cases := [][]fs.OpenOption{
		{&fs.SeekOption{Offset: 3}},
		{&fs.RangeOption{Start: 2, End: 4}},
		{&fs.SeekOption{Offset: 2}, &fs.RangeOption{Start: 2, End: 4}},
		{},
	}
	for _, opts := range cases {
		want := applyReadOptions(base, opts...)
		s, e, ok := logicalSliceAfterOptions(cl, opts...)
		if want == nil {
			require.False(t, ok)
			continue
		}
		require.True(t, ok)
		require.Equal(t, string(want), string(base[s:e]), "opts=%#v", opts)
	}
}

func TestWriteQuorum(t *testing.T) {
	f := &Fs{opt: Options{DataShards: 4, ParityShards: 2}}
	// Default is set by validateOptions; raw Fs opt is not normalized.
	require.Equal(t, 0, f.writeQuorum())

	opt := &Options{Remotes: "a:,b:,c:,d:,e:,f:", DataShards: 4, ParityShards: 2}
	require.NoError(t, validateOptions(opt))
	require.Equal(t, 5, opt.WriteQuorum)

	opt2 := &Options{Remotes: "a:,b:,c:,d:,e:,f:", DataShards: 4, ParityShards: 2, WriteQuorum: 6}
	require.NoError(t, validateOptions(opt2))
	require.Equal(t, 6, opt2.WriteQuorum)
}

type failPutFs struct {
	fs.Fs
	fail bool
}

func (f failPutFs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	if f.fail {
		return nil, errors.New("failPutFs: injected Put failure")
	}
	return f.Fs.Put(ctx, in, src, options...)
}

type failListFs struct {
	fs.Fs
	fail bool
}

func (f failListFs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if f.fail {
		return nil, errors.New("failListFs: injected List failure")
	}
	return f.Fs.List(ctx, dir)
}

type failMkdirFs struct {
	fs.Fs
	fail bool
}

func (f failMkdirFs) Mkdir(ctx context.Context, dir string) error {
	if f.fail {
		return errors.New("failMkdirFs: injected Mkdir failure")
	}
	return f.Fs.Mkdir(ctx, dir)
}

func makeMemoryBackends(t *testing.T, n int, prefix string) []fs.Fs {
	t.Helper()
	ctx := context.Background()
	backends := make([]fs.Fs, n)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:"+prefix+"-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	return backends
}

func makeLocalBackends(t *testing.T, n int, prefix string) []fs.Fs {
	t.Helper()
	ctx := context.Background()
	root := t.TempDir()
	backends := make([]fs.Fs, n)
	for i := range backends {
		p := filepath.Join(root, prefix+"-"+strconv.Itoa(i))
		b, err := cache.Get(ctx, p)
		require.NoError(t, err)
		backends[i] = b
	}
	return backends
}

func TestMkdirQuorum(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 7, "rs-mkdir-quorum")
	backends[6] = failMkdirFs{Fs: backends[6], fail: true}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   4,
			ParityShards: 3,
			WriteQuorum:  6,
		},
		features: (&fs.Features{}),
	}
	require.NoError(t, f.Mkdir(ctx, "qdir"))
	f.opt.WriteQuorum = 7
	require.Error(t, f.Mkdir(ctx, "qdir-all"))
}

func TestRmdirPragmaticAndEmptyChecks(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-rmdir-pragmatic")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   3,
			ParityShards: 1,
			WriteQuorum:  3,
		},
		features: (&fs.Features{}),
	}
	require.NoError(t, backends[0].Mkdir(ctx, "d1"))
	require.NoError(t, backends[1].Mkdir(ctx, "d1"))
	require.NoError(t, f.Rmdir(ctx, "d1"))

	require.NoError(t, backends[0].Mkdir(ctx, "nonempty"))
	_, err := backends[0].Put(ctx, bytes.NewReader([]byte("x")), object.NewStaticObjectInfo("nonempty/a.txt", time.Now(), 1, true, nil, nil))
	require.NoError(t, err)
	err = f.Rmdir(ctx, "nonempty")
	require.Error(t, err)
	require.Contains(t, err.Error(), fs.ErrorDirectoryNotEmpty.Error())
}

func TestPutFailsWhenWriteQuorumNotMet(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 7) // k=4, m=3
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-quorum-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	// Make one backend unavailable for the preflight check.
	backends[6] = failListFs{Fs: backends[6], fail: true}

	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         4,
			ParityShards:       3,
			WriteQuorum:        7, // require all
			Rollback:           true,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	data := []byte("quorum-test")
	srcInfo := object.NewStaticObjectInfo("q.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), srcInfo)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient writable remotes for quorum")

	// With quorum 6, the same single unavailable backend should still allow Put to proceed.
	f.opt.WriteQuorum = 6
	_, err = f.Put(ctx, bytes.NewReader(data), srcInfo)
	require.NoError(t, err)
}

func TestPutStreamingKnownSize(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-put-streaming")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			WriteQuorum:        3,
			Rollback:           true,
			UseSpooling:        false,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	payload := bytes.Repeat([]byte("streaming-put"), 64)
	info := object.NewStaticObjectInfo("stream.bin", time.Unix(1700001234, 0), int64(len(payload)), true, nil, nil)
	obj, err := f.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), obj.Size())

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestPutStreamingCompletesWithinTimeout(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-put-streaming-par")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			WriteQuorum:        3,
			Rollback:           true,
			UseSpooling:        false,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	payload := bytes.Repeat([]byte("par-put"), 64)
	info := object.NewStaticObjectInfo("par.bin", time.Unix(1700001234, 0), int64(len(payload)), true, nil, nil)
	errCh := make(chan error, 1)
	go func() {
		_, err := f.Put(ctx, bytes.NewReader(payload), info)
		errCh <- err
	}()
	select {
	case err := <-errCh:
		require.NoError(t, err)
	case <-time.After(30 * time.Second):
		t.Fatal("Put timed out (streaming Put should complete without deadlock)")
	}
}

func TestPutStreamingUnknownSizeFallsBackToSpooling(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-put-streaming-unknown")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			WriteQuorum:        3,
			Rollback:           true,
			UseSpooling:        false,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	payload := bytes.Repeat([]byte("unknown"), 32)
	info := object.NewStaticObjectInfo("unknown.bin", time.Unix(1700001235, 0), -1, true, nil, nil)
	obj, err := f.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)
	require.Equal(t, int64(len(payload)), obj.Size())

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestPutStreamingRollbackOnShardFailure(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-put-streaming-rollback")
	backends[3] = failPutFs{Fs: backends[3], fail: true}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			WriteQuorum:        3,
			Rollback:           true,
			UseSpooling:        false,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	payload := bytes.Repeat([]byte("rollback"), 64)
	info := object.NewStaticObjectInfo("rollback.bin", time.Unix(1700001236, 0), int64(len(payload)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(payload), info)
	require.Error(t, err)

	for i := 0; i < len(backends)-1; i++ {
		_, err := backends[i].NewObject(ctx, "rollback.bin")
		require.Error(t, err, "rollback should remove uploaded object from shard %d", i)
	}
}

func TestExtractShardPayloadRejectsCorruption(t *testing.T) {
	payload := []byte("hello shard")
	ft := NewRSFooter(int64(len(payload)), nil, nil, time.Unix(1700000000, 0), 2, 1, 0, 64, 1, crc32cChecksum(payload))
	fb, err := ft.MarshalBinary()
	require.NoError(t, err)

	particle := append(append([]byte{}, payload...), fb...)
	okPayload, _, err := ExtractParticlePayload(particle, 0)
	require.NoError(t, err)
	require.Equal(t, payload, okPayload)

	particle[0] ^= 0xFF
	_, _, err = ExtractParticlePayload(particle, 0)
	require.Error(t, err)
}

func TestReconstructDataFromShards(t *testing.T) {
	data := bytes.Repeat([]byte("xyz123"), 200)
	src := object.NewStaticObjectInfo("r.bin", time.Unix(1700001234, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(context.Background(), bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)

	shards := make([][]byte, 4)
	var ref *Footer
	for i := range writers {
		raw := writers[i].Bytes()
		payload, ft, err := ExtractParticlePayload(raw, i)
		require.NoError(t, err)
		shards[i] = payload
		ref = ft
	}
	shards[1] = nil // one missing shard should still reconstruct
	out, err := ReconstructDataFromShards(shards, 2, 2, int64(len(data)), ref.StripeSize, ref.NumStripes)
	require.NoError(t, err)
	require.Equal(t, data, out)

	shards[0] = nil
	shards[2] = nil
	shards[3] = nil
	_, err = ReconstructDataFromShards(shards, 2, 2, int64(len(data)), ref.StripeSize, ref.NumStripes)
	require.Error(t, err)
}

func TestStatusHealRebuildCommands(t *testing.T) {
	f := &Fs{
		opt: Options{
			DataShards:   2,
			ParityShards: 1,
		},
	}
	statusAny, err := f.statusCommand(context.Background(), nil, nil)
	require.NoError(t, err)
	status := statusAny.(string)
	require.True(t, strings.Contains(status, "Write quorum"))

	healAny, err := f.healCommand(context.Background(), []string{"p/a.txt"}, nil)
	require.NoError(t, err)
	require.True(t, strings.Contains(healAny.(string), "RS heal completed"), healAny.(string))
}

func TestReconstructMissingShardsHelper(t *testing.T) {
	data := bytes.Repeat([]byte("abc123"), 150)
	src := object.NewStaticObjectInfo("h.bin", time.Unix(1700002000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(context.Background(), bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)

	shards := make([][]byte, 4)
	var ref *Footer
	for i := range writers {
		payload, ft, err := ExtractParticlePayload(writers[i].Bytes(), i)
		require.NoError(t, err)
		shards[i] = payload
		ref = ft
	}
	shards[1] = nil
	out, err := reconstructMissingShards(shards, 2, 2, ref.StripeSize, ref.NumStripes)
	require.NoError(t, err)
	require.NotNil(t, out[1])
}

func TestHealStripeWiseMatchesReconstructMissingShards(t *testing.T) {
	ctx := context.Background()
	data := bytes.Repeat([]byte("equiv-heal-"), 120)
	src := object.NewStaticObjectInfo("equiv.bin", time.Unix(1700006000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)

	shards := make([][]byte, 4)
	var ref *Footer
	for i := range writers {
		payload, ft, err := ExtractParticlePayload(writers[i].Bytes(), i)
		require.NoError(t, err)
		shards[i] = payload
		ref = ft
	}

	shardsCopy := make([][]byte, 4)
	copy(shardsCopy, shards)
	golden1 := append([]byte(nil), shards[1]...)
	golden3 := append([]byte(nil), shards[3]...)
	shardsCopy[1] = nil
	shardsCopy[3] = nil

	outBuf, err := ReconstructMissingShardPayloadsStripeWiseForTest(shardsCopy, 2, 2, ref.StripeSize, ref.NumStripes, []int{1, 3})
	require.NoError(t, err)
	require.Equal(t, golden1, outBuf[1], "shard 1 payload mismatch")
	require.Equal(t, golden3, outBuf[3], "shard 3 payload mismatch")
}

func TestRebuildMissingShardsForObjectEndToEnd(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-e2e-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}

	data := bytes.Repeat([]byte("rebuild-me"), 256)
	srcInfo := object.NewStaticObjectInfo("obj.bin", time.Unix(1700003000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, 0, ios, true)
	require.NoError(t, err)

	for i := range backends {
		blob := writers[i].Bytes()
		info := object.NewStaticObjectInfo("obj.bin", srcInfo.ModTime(ctx), int64(len(blob)), true, nil, nil)
		_, err := backends[i].Put(ctx, bytes.NewReader(blob), info)
		require.NoError(t, err)
	}

	// Simulate one missing shard
	objMissing, err := backends[3].NewObject(ctx, "obj.bin")
	require.NoError(t, err)
	require.NoError(t, objMissing.Remove(ctx))

	restored, err := f.rebuildMissingShardsForObject(ctx, "obj.bin", false)
	require.NoError(t, err)
	require.Equal(t, 1, restored)

	obj, err := f.NewObject(ctx, "obj.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, data, got)
}

func TestRebuildMissingShardsForObjectDryRun(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-dry-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}

	data := bytes.Repeat([]byte("dry-run"), 128)
	srcInfo := object.NewStaticObjectInfo("dry.bin", time.Unix(1700004000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, 0, ios, true)
	require.NoError(t, err)

	for i := range backends {
		blob := writers[i].Bytes()
		info := object.NewStaticObjectInfo("dry.bin", srcInfo.ModTime(ctx), int64(len(blob)), true, nil, nil)
		_, err := backends[i].Put(ctx, bytes.NewReader(blob), info)
		require.NoError(t, err)
	}

	objMissing, err := backends[3].NewObject(ctx, "dry.bin")
	require.NoError(t, err)
	require.NoError(t, objMissing.Remove(ctx))

	n, err := f.rebuildMissingShardsForObject(ctx, "dry.bin", true)
	require.NoError(t, err)
	require.Equal(t, 1, n)

	_, err = backends[3].NewObject(ctx, "dry.bin")
	require.Error(t, err, "dry-run must not upload missing shard")
}

// sequenceReader produces a deterministic pseudo-random byte stream of a fixed length.
type sequenceReader struct {
	remaining int64
	r         *rand.Rand
}

func (s *sequenceReader) Read(p []byte) (int, error) {
	if s.remaining == 0 {
		return 0, io.EOF
	}
	n := len(p)
	if int64(n) > s.remaining {
		n = int(s.remaining)
	}
	buf := p[:n]
	for i := range buf {
		buf[i] = byte(s.r.Intn(256))
	}
	s.remaining -= int64(n)
	return n, nil
}

// TestLargeObjectPutOpen1GiB exercises the upload/read path with a ~1 GiB object.
// It streams deterministic data into Put and verifies the logical content via MD5 and footer fields.
func TestLargeObjectPutOpen1GiB(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping 1 GiB rs test in short mode")
	}

	const size int64 = 1 << 30 // 1 GiB
	const seed int64 = 12345

	ctx := context.Background()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-large-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}

	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}

	modTime := time.Unix(1800000001, 0)
	srcInfo := object.NewStaticObjectInfo("large.bin", modTime, size, true, nil, nil)

	// Stream 1 GiB into Put using a deterministic sequence.
	putReader := &sequenceReader{
		remaining: size,
		r:         rand.New(rand.NewSource(seed)),
	}
	obj, err := f.Put(ctx, putReader, srcInfo)
	require.NoError(t, err)
	require.Equal(t, size, obj.Size())

	// Compute expected MD5 over the same sequence (independent of footer MD5).
	expMD5 := md5.New()
	_, err = io.Copy(expMD5, &sequenceReader{
		remaining: size,
		r:         rand.New(rand.NewSource(seed)),
	})
	require.NoError(t, err)
	expMD5Hex := hex.EncodeToString(expMD5.Sum(nil))

	// Verify logical MD5 from rs footer matches expected.
	gotHash, err := obj.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	require.Equal(t, expMD5Hex, gotHash)

	// Read back the object via Open and re-hash to ensure end-to-end integrity.
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()

	gotMD5 := md5.New()
	_, err = io.Copy(gotMD5, rc)
	require.NoError(t, err)
	gotMD5Hex := hex.EncodeToString(gotMD5.Sum(nil))

	require.Equal(t, expMD5Hex, gotMD5Hex)
}

func TestSetModTimeUpdatesFooters(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-mtime-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}

	data := bytes.Repeat([]byte("mtime"), 333)
	old := time.Unix(1700005000, 0)
	srcInfo := object.NewStaticObjectInfo("mt.bin", old, int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, 0, ios, true)
	require.NoError(t, err)

	for i := range backends {
		blob := writers[i].Bytes()
		info := object.NewStaticObjectInfo("mt.bin", old, int64(len(blob)), true, nil, nil)
		_, err := backends[i].Put(ctx, bytes.NewReader(blob), info)
		require.NoError(t, err)
	}

	obj, err := f.NewObject(ctx, "mt.bin")
	require.NoError(t, err)

	newt := time.Unix(1800000000, 0)
	require.NoError(t, obj.SetModTime(ctx, newt))
	require.Equal(t, newt.Unix(), obj.ModTime(ctx).Unix())

	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, data, got)

	for i := range backends {
		sobj, err := backends[i].NewObject(ctx, "mt.bin")
		require.NoError(t, err)
		raw, err := sobj.Open(ctx)
		require.NoError(t, err)
		all, err := io.ReadAll(raw)
		_ = raw.Close()
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(all), FooterSize)
		ft, err := ParseFooter(all[len(all)-FooterSize:])
		require.NoError(t, err)
		require.Equal(t, newt.Unix(), ft.Mtime)
		payload := all[:len(all)-FooterSize]
		require.Equal(t, crc32cChecksum(payload), ft.PayloadCRC32C)
	}
}

func TestHealCommandSummaryCounts(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 4)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-heal-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         2,
			ParityShards:       2,
			Rollback:           true,
			UseSpooling:        true,
		},
		features: (&fs.Features{}),
	}

	// Object A: healthy (all shards present) -> skipped
	objA := "healthy.bin"
	dataA := bytes.Repeat([]byte("A"), 1024)
	writeObjectShardsForTest(ctx, t, backends, objA, dataA)

	// Object B: one missing shard -> healed
	objB := "heal.bin"
	dataB := bytes.Repeat([]byte("B"), 2048)
	writeObjectShardsForTest(ctx, t, backends, objB, dataB)
	missingObj, err := backends[2].NewObject(ctx, objB)
	require.NoError(t, err)
	require.NoError(t, missingObj.Remove(ctx))

	// Object C: only one shard left -> failed
	objC := "fail.bin"
	dataC := bytes.Repeat([]byte("C"), 4096)
	writeObjectShardsForTest(ctx, t, backends, objC, dataC)
	for i := 1; i < 4; i++ {
		o, err := backends[i].NewObject(ctx, objC)
		require.NoError(t, err)
		require.NoError(t, o.Remove(ctx))
	}

	outAny, err := f.healCommand(ctx, nil, nil)
	require.NoError(t, err)
	out := outAny.(string)
	require.True(t, strings.Contains(out, "Scanned: 3"), out)
	require.True(t, strings.Contains(out, "Healed: 1"), out)
	require.True(t, strings.Contains(out, "Healthy/Skipped: 1"), out)
	require.True(t, strings.Contains(out, "Failed: 1"), out)
	require.True(t, strings.Contains(out, "Failed remotes:"), out)
	require.True(t, strings.Contains(out, "fail.bin"), out)
}

func TestListAllObjectRemotesSorted(t *testing.T) {
	ctx := context.Background()
	backends := make([]fs.Fs, 2)
	for i := range backends {
		b, err := cache.Get(ctx, ":memory:rs-sort-"+time.Now().Format("150405")+"-"+string(rune('a'+i)))
		require.NoError(t, err)
		backends[i] = b
	}
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   1,
			ParityShards: 1,
		},
		features: (&fs.Features{}),
	}

	for _, name := range []string{"z-last.bin", "a-first.bin", "m-middle.bin"} {
		info := object.NewStaticObjectInfo(name, time.Unix(1700003000, 0), int64(len(name)), true, nil, nil)
		_, err := backends[0].Put(ctx, bytes.NewReader([]byte(name)), info)
		require.NoError(t, err)
	}
	remotes, err := f.listAllObjectRemotes(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{"a-first.bin", "m-middle.bin", "z-last.bin"}, remotes)
}

func writeObjectShardsForTest(ctx context.Context, t *testing.T, backends []fs.Fs, remote string, data []byte) {
	t.Helper()
	src := object.NewStaticObjectInfo(remote, time.Unix(1700003000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, len(backends))
	ios := make([]io.Writer, len(backends))
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, 0, ios, true)
	require.NoError(t, err)
	for i := range backends {
		blob := writers[i].Bytes()
		info := object.NewStaticObjectInfo(remote, src.ModTime(ctx), int64(len(blob)), true, nil, nil)
		_, err := backends[i].Put(ctx, bytes.NewReader(blob), info)
		require.NoError(t, err)
	}
}

func TestNewFsPutOpenIntegration(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-int-" + unique + "-a",
		":memory:rs-int-" + unique + "-b",
		":memory:rs-int-" + unique + "-c",
		":memory:rs-int-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"use_spooling":         "true",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-integration", "", cfg)
	require.NoError(t, err)

	payload := bytes.Repeat([]byte("integration-data"), 300)
	info := object.NewStaticObjectInfo("int.bin", time.Unix(1700004000, 0), int64(len(payload)), true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	obj, err := fsi.NewObject(ctx, "int.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload, got)

	rcRange, err := obj.Open(ctx, &fs.RangeOption{Start: 100, End: 199})
	require.NoError(t, err)
	gotRange, err := io.ReadAll(rcRange)
	_ = rcRange.Close()
	require.NoError(t, err)
	require.Equal(t, payload[100:200], gotRange)
}

// TestOpenStripeStreamingSmallReads exercises stripe-wise Open with tiny Read buffers (data-shard path).
func TestOpenStripeStreamingSmallReads(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-stream-" + unique + "-a",
		":memory:rs-stream-" + unique + "-b",
		":memory:rs-stream-" + unique + "-c",
		":memory:rs-stream-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"use_spooling":         "true",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-stream-small", "", cfg)
	require.NoError(t, err)

	payload := bytes.Repeat([]byte("stream-chunk-"), 40)
	info := object.NewStaticObjectInfo("stream.bin", time.Unix(1700005000, 0), int64(len(payload)), true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	obj, err := fsi.NewObject(ctx, "stream.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })

	var got bytes.Buffer
	buf := make([]byte, 7)
	for {
		n, err := rc.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			require.Equal(t, io.EOF, err)
			break
		}
	}
	require.Equal(t, payload, got.Bytes())
}

// TestOpenFullReconstructStreamingSmallReads drops one data shard and reads logical content in small chunks.
func TestOpenFullReconstructStreamingSmallReads(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-degraded-" + unique + "-a",
		":memory:rs-degraded-" + unique + "-b",
		":memory:rs-degraded-" + unique + "-c",
		":memory:rs-degraded-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"use_spooling":         "true",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-degraded-stream", "", cfg)
	require.NoError(t, err)
	f := fsi.(*Fs)

	payload := bytes.Repeat([]byte("degraded-stream-"), 35)
	info := object.NewStaticObjectInfo("deg.bin", time.Unix(1700005100, 0), int64(len(payload)), true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	// Remove data shard 0 — Open must use full stripe reconstruction, not data-shards-only join.
	o0, err := f.backends[0].NewObject(ctx, "deg.bin")
	require.NoError(t, err)
	require.NoError(t, o0.Remove(ctx))

	obj, err := fsi.NewObject(ctx, "deg.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rc.Close() })

	var got bytes.Buffer
	buf := make([]byte, 11)
	for {
		n, err := rc.Read(buf)
		got.Write(buf[:n])
		if err != nil {
			require.Equal(t, io.EOF, err)
			break
		}
	}
	require.Equal(t, payload, got.Bytes())
}

// TestOpenRangeDegradedStripePath checks RangeOption on the full-reconstruct streaming path.
func TestOpenRangeDegradedStripePath(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-deg-rng-" + unique + "-a",
		":memory:rs-deg-rng-" + unique + "-b",
		":memory:rs-deg-rng-" + unique + "-c",
		":memory:rs-deg-rng-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"use_spooling":         "true",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-deg-range", "", cfg)
	require.NoError(t, err)
	f := fsi.(*Fs)

	payload := bytes.Repeat([]byte("range-deg-"), 50)
	info := object.NewStaticObjectInfo("deg-range.bin", time.Unix(1700005200, 0), int64(len(payload)), true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	o0, err := f.backends[0].NewObject(ctx, "deg-range.bin")
	require.NoError(t, err)
	require.NoError(t, o0.Remove(ctx))

	obj, err := fsi.NewObject(ctx, "deg-range.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx, &fs.RangeOption{Start: 40, End: 99})
	require.NoError(t, err)
	gotRange, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload[40:100], gotRange)
}

func TestNewFsUnknownSizeDefaultNoSpoolingFallsBack(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-int-unk-" + unique + "-a",
		":memory:rs-int-unk-" + unique + "-b",
		":memory:rs-int-unk-" + unique + "-c",
		":memory:rs-int-unk-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-int-unk", "", cfg)
	require.NoError(t, err)
	rf := fsi.(*Fs)
	require.False(t, rf.opt.UseSpooling, "use_spooling should default false")

	payload := bytes.Repeat([]byte("unk-default"), 120)
	info := object.NewStaticObjectInfo("unk.bin", time.Unix(1700004100, 0), -1, true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	obj, err := fsi.NewObject(ctx, "unk.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload, got)
	require.Equal(t, int64(len(payload)), obj.Size())
}

func TestNewFsHealIntegration(t *testing.T) {
	ctx := context.Background()
	unique := strconv.FormatInt(time.Now().UnixNano(), 10)
	remotes := []string{
		":memory:rs-rebuild-" + unique + "-a",
		":memory:rs-rebuild-" + unique + "-b",
		":memory:rs-rebuild-" + unique + "-c",
		":memory:rs-rebuild-" + unique + "-d",
	}
	cfg := configmap.Simple{
		"remotes":              strings.Join(remotes, ","),
		"data_shards":          "3",
		"parity_shards":        "1",
		"use_spooling":         "true",
		"rollback":             "true",
	}
	fsi, err := NewFs(ctx, "rs-rebuild", "", cfg)
	require.NoError(t, err)
	f := fsi.(*Fs)

	payload := bytes.Repeat([]byte("rebuild-integration"), 200)
	info := object.NewStaticObjectInfo("rebuild.bin", time.Unix(1700004100, 0), int64(len(payload)), true, nil, nil)
	_, err = fsi.Put(ctx, bytes.NewReader(payload), info)
	require.NoError(t, err)

	// Delete one shard and rebuild it via backend command.
	missingObj, err := f.backends[3].NewObject(ctx, "rebuild.bin")
	require.NoError(t, err)
	require.NoError(t, missingObj.Remove(ctx))

	out, err := f.Command(ctx, "heal", []string{"rebuild.bin"}, nil)
	require.NoError(t, err)
	require.True(t, strings.Contains(out.(string), "restored 1 shard"))

	obj, err := fsi.NewObject(ctx, "rebuild.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	got, err := io.ReadAll(rc)
	_ = rc.Close()
	require.NoError(t, err)
	require.Equal(t, payload, got)
}

func TestMultiStripeEncodeDecode(t *testing.T) {
	ctx := context.Background()
	const stripeS = 32 // k=2 => 64 logical bytes per stripe; forces multiple stripes for small payloads
	data := bytes.Repeat([]byte("M"), 100)
	src := object.NewStaticObjectInfo("multi.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, 4)
	ios := make([]io.Writer, 4)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	res, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, stripeS, ios, true)
	require.NoError(t, err)
	require.Greater(t, res.NumStripes, uint32(1))

	shards := make([][]byte, 4)
	var ref *Footer
	for i := range writers {
		payload, ft, err := ExtractParticlePayload(writers[i].Bytes(), i)
		require.NoError(t, err)
		shards[i] = payload
		ref = ft
	}
	out, err := ReconstructDataFromShards(shards, 2, 2, int64(len(data)), ref.StripeSize, ref.NumStripes)
	require.NoError(t, err)
	require.Equal(t, data, out)
}

func TestNewFsValidatesRemoteCount(t *testing.T) {
	ctx := context.Background()
	cfg := configmap.Simple{
		"remotes":       ":memory:a,:memory:b",
		"data_shards":   "2",
		"parity_shards": "1",
	}
	_, err := NewFs(ctx, "rs-bad", "", cfg)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "remotes count must equal"))
}

func TestListDirectoryQuorumMerge(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-list-quorum")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	keep := object.NewStaticObjectInfo("keep.bin", time.Unix(1700000000, 0), int64(len("keep")), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader([]byte("keep")), keep)
	require.NoError(t, err)
	drop := object.NewStaticObjectInfo("drop.bin", time.Unix(1700000001, 0), int64(len("drop")), true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader([]byte("drop")), drop)
	require.NoError(t, err)
	o, err := backends[3].NewObject(ctx, "keep.bin")
	require.NoError(t, err)
	require.NoError(t, o.Remove(ctx))
	o, err = backends[2].NewObject(ctx, "drop.bin")
	require.NoError(t, err)
	require.NoError(t, o.Remove(ctx))
	o, err = backends[3].NewObject(ctx, "drop.bin")
	require.NoError(t, err)
	require.NoError(t, o.Remove(ctx))
	entries, err := f.List(ctx, "")
	require.NoError(t, err)
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Remote())
	}
	require.Equal(t, []string{"keep.bin"}, names)
}

func TestRemoveAndSetModTimeQuorum(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-remove-modtime")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			Rollback:           true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("hello quorum")
	info := object.NewStaticObjectInfo("x.bin", time.Unix(1700004000, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	// Remove one shard object to force degraded-but-quorum behavior.
	missingObj, err := backends[3].NewObject(ctx, "x.bin")
	require.NoError(t, err)
	require.NoError(t, missingObj.Remove(ctx))

	o, err := f.NewObject(ctx, "x.bin")
	require.NoError(t, err)
	require.NoError(t, o.SetModTime(ctx, time.Unix(1700005000, 0)))
	require.NoError(t, o.Remove(ctx))

	f.opt.WriteQuorum = 4
	_, err = f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
	o2, err := f.NewObject(ctx, "x.bin")
	require.NoError(t, err)
	missingObj2, err := backends[3].NewObject(ctx, "x.bin")
	require.NoError(t, err)
	require.NoError(t, missingObj2.Remove(ctx))
	require.Error(t, o2.SetModTime(ctx, time.Unix(1700006000, 0)))
	require.Error(t, o2.Remove(ctx))
}

func TestDegradedCommand(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-degraded-cmd")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			Rollback:           true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("degraded")
	info := object.NewStaticObjectInfo("d.bin", time.Unix(1700004000, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)
	one, err := backends[3].NewObject(ctx, "d.bin")
	require.NoError(t, err)
	require.NoError(t, one.Remove(ctx))
	two, err := backends[2].NewObject(ctx, "d.bin")
	require.NoError(t, err)
	require.NoError(t, two.Remove(ctx))

	out, err := f.Command(ctx, "degraded", []string{"summary"}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "Degraded: 1")

	out, err = f.Command(ctx, "degraded", []string{"ls"}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "DEGRADED d.bin")

	out, err = f.Command(ctx, "degraded", []string{"lsd"}, nil)
	require.NoError(t, err)
	require.Contains(t, out.(string), "not implemented")
}

func TestCopyMoveServerSide(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-copy-move")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			Rollback:           true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	data := []byte("copy-move-data")
	info := object.NewStaticObjectInfo("src.bin", time.Unix(1700004000, 0), int64(len(data)), true, nil, nil)
	_, err := f.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	srcObj, err := f.NewObject(ctx, "src.bin")
	require.NoError(t, err)

	// Degrade one shard of source object and ensure copy still succeeds at quorum.
	degradedShardObj, err := backends[3].NewObject(ctx, "src.bin")
	require.NoError(t, err)
	require.NoError(t, degradedShardObj.Remove(ctx))

	copied, err := f.Copy(ctx, srcObj, "copy.bin")
	require.NoError(t, err)
	require.Equal(t, "copy.bin", copied.Remote())

	moved, err := f.Move(ctx, srcObj, "moved.bin")
	require.NoError(t, err)
	require.Equal(t, "moved.bin", moved.Remote())
	_, err = f.NewObject(ctx, "src.bin")
	require.Error(t, err)
}

func TestCopyMoveRejectIncompatibleSource(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-copy-incompatible")
	f := &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:   3,
			ParityShards: 1,
			WriteQuorum:  3,
		},
		features: (&fs.Features{}),
	}
	mem, err := cache.Get(ctx, ":memory:rs-foreign-"+strconv.FormatInt(time.Now().UnixNano(), 10))
	require.NoError(t, err)
	foreignInfo := object.NewStaticObjectInfo("foreign.bin", time.Unix(1700004000, 0), 1, true, nil, nil)
	foreignObj, err := mem.Put(ctx, bytes.NewReader([]byte("x")), foreignInfo)
	require.NoError(t, err)
	_, err = f.Copy(ctx, foreignObj, "dst.bin")
	require.ErrorIs(t, err, fs.ErrorCantCopy)
	_, err = f.Move(ctx, foreignObj, "dst.bin")
	require.ErrorIs(t, err, fs.ErrorCantMove)
}

func TestDirMoveServerSide(t *testing.T) {
	ctx := context.Background()
	srcBackends := makeLocalBackends(t, 4, "rs-dirmove-src")
	dstBackends := makeLocalBackends(t, 4, "rs-dirmove-dst")
	src := &Fs{
		name:     "rs-src",
		root:     "",
		backends: srcBackends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			Rollback:           true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}
	dst := &Fs{
		name:     "rs-dst",
		root:     "",
		backends: dstBackends,
		opt: Options{
			DataShards:         3,
			ParityShards:       1,
			WriteQuorum:        3,
			UseSpooling:        true,
			Rollback:           true,
			StripeFragmentSize: 64,
		},
		features: (&fs.Features{}),
	}

	data := []byte("dir-move-data")
	info := object.NewStaticObjectInfo("srcdir/a.txt", time.Unix(1700004000, 0), int64(len(data)), true, nil, nil)
	_, err := src.Put(ctx, bytes.NewReader(data), info)
	require.NoError(t, err)

	err = dst.DirMove(ctx, src, "srcdir", "dstdir")
	require.NoError(t, err)
	_, err = dst.NewObject(ctx, "dstdir/a.txt")
	require.NoError(t, err)
	_, err = src.NewObject(ctx, "srcdir/a.txt")
	require.Error(t, err)
}
