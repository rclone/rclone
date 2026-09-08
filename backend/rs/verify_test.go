package rs

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func verifyFsForTest(t *testing.T, backends []fs.Fs, k, m, stripeS int) *Fs {
	t.Helper()
	return &Fs{
		name:     "rs",
		root:     "",
		backends: backends,
		opt: Options{
			DataShards:         k,
			ParityShards:       m,
			UseSpooling:        true,
			StripeFragmentSize: stripeS,
		},
		features: (&fs.Features{}),
	}
}

func runVerify(t *testing.T, f *Fs, remote string, opt map[string]string) string {
	t.Helper()
	out, err := f.Command(context.Background(), "verify", []string{remote}, opt)
	require.NoError(t, err)
	return out.(string)
}

func runVerifyAll(t *testing.T, f *Fs, opt map[string]string) string {
	t.Helper()
	out, err := f.Command(context.Background(), "verify", nil, opt)
	require.NoError(t, err)
	return out.(string)
}

func TestVerifyHealthyObject(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-ok")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "ok.bin"
	data := bytes.Repeat([]byte("verify-ok"), 40)
	writeObjectShardsForTest(ctx, t, backends, remote, data)

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "OK       "+remote)
	require.Contains(t, out, "Failed: 0")
}

func TestVerifyDegradedOneShardMissing(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-degraded")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "deg.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, []byte("degraded-object"))

	obj, err := backends[3].NewObject(ctx, remote)
	require.NoError(t, err)
	require.NoError(t, obj.Remove(ctx))

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "DEGRADED "+remote)
	require.Contains(t, out, "Failed: 0")

	outStrict := runVerify(t, f, remote, map[string]string{"strict": "true"})
	require.Contains(t, outStrict, "DEGRADED "+remote)
	require.Contains(t, outStrict, "Failed: 1")
}

func TestVerifyPayloadCorruptStaleCRC(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-corrupt")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "corrupt.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, bytes.Repeat([]byte("rot"), 50))

	obj, err := backends[0].NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	particle, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	particle[0] ^= 0xff
	require.NoError(t, obj.Remove(ctx))
	info := object.NewStaticObjectInfo(remote, obj.ModTime(ctx), int64(len(particle)), true, nil, nil)
	_, err = backends[0].Put(ctx, bytes.NewReader(particle), info)
	require.NoError(t, err)

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "CORRUPT  "+remote)
	require.Contains(t, out, "PayloadCRC32C mismatch")
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyWriteIDSkew(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-skew")
	f := verifyFsForTest(t, backends, 3, 1, 64)
	remote := "skew.bin"

	dataA := bytes.Repeat([]byte("AAAA"), 64)
	dataB := bytes.Repeat([]byte("BBBB"), 64)
	bufsA, _ := encodeShardsForTest(t, remote, dataA, 3, 1, 64)
	bufsB, _ := encodeShardsForTest(t, remote, dataB, 3, 1, 64)
	mixed := cloneShardBuffers(bufsA)
	mixed[0] = bytes.NewBuffer(bufsB[0].Bytes())
	mixed[1] = bytes.NewBuffer(bufsB[1].Bytes())
	uploadShardBuffers(ctx, t, backends, remote, mixed, time.Unix(1700003000, 0))

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "SKEW     "+remote)
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyTruncatedFooter(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-trunc")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "trunc.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, []byte("truncate-me"))

	obj, err := backends[1].NewObject(ctx, remote)
	require.NoError(t, err)
	require.NoError(t, obj.Remove(ctx))
	info := object.NewStaticObjectInfo(remote, time.Unix(1700003000, 0), 10, true, nil, nil)
	_, err = backends[1].Put(ctx, bytes.NewReader([]byte("short")), info)
	require.NoError(t, err)

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "FAIL     "+remote)
	require.Contains(t, out, "truncated footer")
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyFooterKMMismatch(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-km")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "km.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, []byte("km-check"))

	tamperShardFooterKM(ctx, t, backends, 0, remote, 3, 1)

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "FAIL     "+remote)
	require.Contains(t, out, "footer k/m=")
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyEmptyObject(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-empty")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "empty.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, nil)

	out := runVerify(t, f, remote, nil)
	require.Contains(t, out, "OK       "+remote)
	require.Contains(t, out, "Failed: 0")
}

func TestVerifyHashesHealthy(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-hashes")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "hash-ok.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, bytes.Repeat([]byte("hash"), 60))

	out := runVerify(t, f, remote, map[string]string{"hashes": "true"})
	require.Contains(t, out, "OK       "+remote)
	require.Contains(t, out, "Failed: 0")
}

func TestVerifyHashesMismatchWithFixedCRC(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-hash-mis")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	remote := "hash-bad.bin"
	writeObjectShardsForTest(ctx, t, backends, remote, bytes.Repeat([]byte("hash-bad"), 60))

	obj, err := backends[0].NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	particle, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	particle[0] ^= 0xff
	payload := particle[:len(particle)-FooterSize]
	ft, err := ParseFooter(particle[len(particle)-FooterSize:])
	require.NoError(t, err)
	ft.PayloadCRC32C = crc32cChecksum(payload)
	fb, err := ft.MarshalBinary()
	require.NoError(t, err)
	copy(particle[len(particle)-FooterSize:], fb)
	require.NoError(t, obj.Remove(ctx))
	info := object.NewStaticObjectInfo(remote, obj.ModTime(ctx), int64(len(particle)), true, nil, nil)
	_, err = backends[0].Put(ctx, bytes.NewReader(particle), info)
	require.NoError(t, err)

	out := runVerify(t, f, remote, map[string]string{"hashes": "true"})
	require.Contains(t, out, "CORRUPT  "+remote)
	require.True(t, strings.Contains(out, "MD5 mismatch") || strings.Contains(out, "SHA256 mismatch"))
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyNamespaceScan(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-ns")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	writeObjectShardsForTest(ctx, t, backends, "good-a.bin", []byte("a"))
	writeObjectShardsForTest(ctx, t, backends, "good-b.bin", []byte("b"))
	writeObjectShardsForTest(ctx, t, backends, "bad.bin", bytes.Repeat([]byte("x"), 40))

	obj, err := backends[0].NewObject(ctx, "bad.bin")
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	particle, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())
	particle[5] ^= 0xff
	require.NoError(t, obj.Remove(ctx))
	info := object.NewStaticObjectInfo("bad.bin", obj.ModTime(ctx), int64(len(particle)), true, nil, nil)
	_, err = backends[0].Put(ctx, bytes.NewReader(particle), info)
	require.NoError(t, err)

	out := runVerifyAll(t, f, nil)
	require.Contains(t, out, "Scanned: 3")
	require.Contains(t, out, "OK: 2")
	require.Contains(t, out, "CORRUPT: 1")
	require.Contains(t, out, "Failed: 1")
}

func TestVerifyUnknownOption(t *testing.T) {
	ctx := context.Background()
	backends := makeMemoryBackends(t, 4, "rs-verify-opt")
	f := verifyFsForTest(t, backends, 2, 2, 64)
	_, err := f.Command(ctx, "verify", []string{"x.bin"}, map[string]string{"nope": "true"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown verify option")
}

func tamperShardFooterKM(ctx context.Context, t *testing.T, backends []fs.Fs, shard int, remote string, newK, newM int) {
	t.Helper()
	obj, err := backends[shard].NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := obj.Open(ctx)
	require.NoError(t, err)
	particle, err := io.ReadAll(rc)
	require.NoError(t, err)
	require.NoError(t, rc.Close())

	ft, err := ParseFooter(particle[len(particle)-FooterSize:])
	require.NoError(t, err)
	ft.DataShards = uint8(newK)
	ft.ParityShards = uint8(newM)
	fb, err := ft.MarshalBinary()
	require.NoError(t, err)
	copy(particle[len(particle)-FooterSize:], fb)

	require.NoError(t, obj.Remove(ctx))
	info := object.NewStaticObjectInfo(remote, obj.ModTime(ctx), int64(len(particle)), true, nil, nil)
	_, err = backends[shard].Put(ctx, bytes.NewReader(particle), info)
	require.NoError(t, err)
}
