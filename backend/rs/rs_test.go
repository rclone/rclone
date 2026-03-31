package rs

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"io"
	"math/rand"
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
		MaxParallelUploads: 0,
	}
	require.NoError(t, validateOptions(opt))
	require.Equal(t, 1, opt.MaxParallelUploads)
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

	res, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, ios, true)
	require.NoError(t, err)
	require.Equal(t, int64(len(data)), res.ContentLength)

	for i := range writers {
		raw := writers[i].Bytes()
		require.GreaterOrEqual(t, len(raw), FooterSize)
		payload := raw[:len(raw)-FooterSize]
		ft, err := ParseFooter(raw[len(raw)-FooterSize:])
		require.NoError(t, err)
		require.Equal(t, uint8(2), ft.DataShards)
		require.Equal(t, uint8(2), ft.ParityShards)
		require.Equal(t, uint8(i), ft.CurrentShard)
		require.Equal(t, AlgorithmRS, ft.Algorithm)
		require.Equal(t, crc32cChecksum(payload), ft.PayloadCRC32C)
	}
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
	require.Equal(t, 5, f.writeQuorum())
}

func TestExtractShardPayloadRejectsCorruption(t *testing.T) {
	payload := []byte("hello shard")
	ft := NewRSFooter(int64(len(payload)), nil, nil, time.Unix(1700000000, 0), 2, 1, 0, 64, crc32cChecksum(payload))
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
	_, err := BuildRSShardsToWriters(context.Background(), bytes.NewReader(data), src, 2, 2, ios, true)
	require.NoError(t, err)

	shards := make([][]byte, 4)
	for i := range writers {
		raw := writers[i].Bytes()
		payload, _, err := ExtractParticlePayload(raw, i)
		require.NoError(t, err)
		shards[i] = payload
	}
	shards[1] = nil // one missing shard should still reconstruct
	out, err := ReconstructDataFromShards(shards, 2, 2, int64(len(data)))
	require.NoError(t, err)
	require.Equal(t, data, out)

	shards[0] = nil
	shards[2] = nil
	shards[3] = nil
	_, err = ReconstructDataFromShards(shards, 2, 2, int64(len(data)))
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
	_, err := BuildRSShardsToWriters(context.Background(), bytes.NewReader(data), src, 2, 2, ios, true)
	require.NoError(t, err)

	shards := make([][]byte, 4)
	for i := range writers {
		payload, _, err := ExtractParticlePayload(writers[i].Bytes(), i)
		require.NoError(t, err)
		shards[i] = payload
	}
	shards[1] = nil
	out, err := reconstructMissingShards(shards, 2, 2)
	require.NoError(t, err)
	require.NotNil(t, out[1])
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
			MaxParallelUploads: 2,
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
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, ios, true)
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
			MaxParallelUploads: 2,
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
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, ios, true)
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
			MaxParallelUploads: 2,
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
			MaxParallelUploads: 2,
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
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), srcInfo, 2, 2, ios, true)
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
			MaxParallelUploads: 2,
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
	_, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, 2, 2, ios, true)
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
		"data_shards":          "2",
		"parity_shards":        "2",
		"use_spooling":         "true",
		"rollback":             "true",
		"max_parallel_uploads": "2",
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
		"data_shards":          "2",
		"parity_shards":        "2",
		"use_spooling":         "true",
		"rollback":             "true",
		"max_parallel_uploads": "2",
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

func TestNewFsValidatesRemoteCount(t *testing.T) {
	ctx := context.Background()
	cfg := configmap.Simple{
		"remotes":       ":memory:a,:memory:b,:memory:c",
		"data_shards":   "2",
		"parity_shards": "2",
	}
	_, err := NewFs(ctx, "rs-bad", "", cfg)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "remotes count must equal"))
}
