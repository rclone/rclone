package crypt

import (
	"bytes"
	"context"
	"crypto/md5"
	"fmt"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest/mockfs"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Create a temporary local fs to upload things from

func makeTempLocalFs(t *testing.T) (localFs fs.Fs) {
	localFs, err := fs.TemporaryLocalFs(context.Background())
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, localFs.Rmdir(context.Background(), ""))
	})
	return localFs
}

// Upload a file to a remote
func uploadFile(t *testing.T, f fs.Fs, remote, contents string) (obj fs.Object) {
	inBuf := bytes.NewBufferString(contents)
	t1 := time.Date(2012, time.December, 17, 18, 32, 31, 0, time.UTC)
	upSrc := object.NewStaticObjectInfo(remote, t1, int64(len(contents)), true, nil, nil)
	obj, err := f.Put(context.Background(), inBuf, upSrc)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, obj.Remove(context.Background()))
	})
	return obj
}

// Test the ObjectInfo
func testObjectInfo(t *testing.T, f *Fs, wrap bool) {
	var (
		contents = random.String(100)
		path     = "hash_test_object"
		ctx      = context.Background()
	)
	if wrap {
		path = "_wrap"
	}

	localFs := makeTempLocalFs(t)

	obj := uploadFile(t, localFs, path, contents)

	// encrypt the data
	inBuf := bytes.NewBufferString(contents)
	var outBuf bytes.Buffer
	enc, err := f.cipher.newEncrypter(inBuf, nil)
	require.NoError(t, err)
	nonce := enc.nonce // read the nonce at the start
	_, err = io.Copy(&outBuf, enc)
	require.NoError(t, err)

	var oi fs.ObjectInfo = obj
	if wrap {
		// wrap the object in an fs.ObjectUnwrapper if required
		oi = fs.NewOverrideRemote(oi, "new_remote")
	}

	// wrap the object in a crypt for upload using the nonce we
	// saved from the encrypter
	src := f.newObjectInfo(oi, nonce)

	// Test ObjectInfo methods
	if !f.opt.NoDataEncryption {
		assert.Equal(t, int64(outBuf.Len()), src.Size())
	}
	assert.Equal(t, f, src.Fs())
	assert.NotEqual(t, path, src.Remote())

	// Test ObjectInfo.Hash
	wantHash := md5.Sum(outBuf.Bytes())
	if f.opt.NoDataEncryption {
		// If the data isn't encrypted, the hash should be that of the plaintext
		wantHash = md5.Sum([]byte(contents))
	}
	gotHash, err := src.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%x", wantHash), gotHash)
}

func testComputeHash(t *testing.T, f *Fs) {
	var (
		contents = random.String(100)
		path     = "compute_hash_test"
		ctx      = context.Background()
		hashType = f.Fs.Hashes().GetOne()
	)

	if hashType == hash.None {
		t.Skipf("%v: does not support hashes", f.Fs)
	}

	localFs := makeTempLocalFs(t)

	// Upload a file to localFs as a test object
	localObj := uploadFile(t, localFs, path, contents)

	// Upload the same data to the remote Fs also
	remoteObj := uploadFile(t, f, path, contents)

	// Calculate the expected Hash of the remote object
	computedHash, err := f.ComputeHash(ctx, remoteObj.(*Object), localObj, hashType)
	require.NoError(t, err)

	// Test computed hash matches remote object hash
	remoteObjHash, err := remoteObj.(*Object).Object.Hash(ctx, hashType)
	require.NoError(t, err)
	assert.Equal(t, remoteObjHash, computedHash)
}

// memChunkWriter is a fs.ChunkWriter which keeps the chunks in memory.
//
// It reads each chunk twice around a Seek, as the s3 backend does to
// hash a chunk and then upload it.
type memChunkWriter struct {
	mu      sync.Mutex
	chunks  map[int][]byte
	closed  bool
	aborted bool
	src     fs.ObjectInfo // what the wrapped remote was told it is writing
}

func (w *memChunkWriter) WriteChunk(ctx context.Context, chunkNumber int, reader io.ReadSeeker) (int64, error) {
	first, err := io.ReadAll(reader)
	if err != nil {
		return -1, err
	}
	if _, err := reader.Seek(0, io.SeekStart); err != nil {
		return -1, err
	}
	second, err := io.ReadAll(reader)
	if err != nil {
		return -1, err
	}
	if !bytes.Equal(first, second) {
		return -1, fmt.Errorf("chunk %d read differently the second time", chunkNumber)
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.chunks == nil {
		w.chunks = map[int][]byte{}
	}
	w.chunks[chunkNumber] = first
	return int64(len(first)), nil
}

func (w *memChunkWriter) Close(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.closed = true
	return nil
}

func (w *memChunkWriter) Abort(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.aborted = true
	return nil
}

// bytes returns the chunks written so far in chunk order
func (w *memChunkWriter) bytes() []byte {
	w.mu.Lock()
	defer w.mu.Unlock()
	var out []byte
	for i := 0; ; i++ {
		chunk, ok := w.chunks[i]
		if !ok {
			return out
		}
		out = append(out, chunk...)
	}
}

// newChunkTestFs returns a crypt Fs over a mock Fs whose OpenChunkWriter
// hands out mem with the ChunkWriterInfo given, recording what it was
// called with in gotRemote, gotSize and gotOptions. With a zero
// info.ChunkSize the mock takes a ChunkOption as its chunk size, like a
// remote which honours the hint.
func newChunkTestFs(t *testing.T, mem *memChunkWriter, info fs.ChunkWriterInfo, gotRemote *string, gotSize *int64, gotOptions *[]fs.OpenOption) *Fs {
	ctx := context.Background()
	enc, err := NewNameEncoding("base32")
	require.NoError(t, err)
	c, err := newCipher(NameEncryptionStandard, "potato", "", true, enc)
	require.NoError(t, err)
	c.cryptoRand = &zeroes{}
	wrapped, err := mockfs.NewFs(ctx, "mock", "", nil)
	require.NoError(t, err)
	wrapped.Features().OpenChunkWriter = func(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
		*gotRemote = remote
		*gotSize = src.Size()
		*gotOptions = options
		mem.src = src
		// Ask for the checksum of the object, as s3 does to store it
		_, _ = src.Hash(ctx, hash.MD5)
		got := info
		if got.ChunkSize == 0 {
			for _, option := range options {
				if chunkOption, ok := option.(*fs.ChunkOption); ok {
					got.ChunkSize = chunkOption.ChunkSize
				}
			}
		}
		return got, mem, nil
	}
	f := &Fs{
		Fs:     wrapped,
		name:   "crypt",
		cipher: c,
	}
	f.features = (&fs.Features{}).Fill(ctx, f).Mask(ctx, wrapped)
	require.NotNil(t, f.Features().OpenChunkWriter)
	return f
}

func TestOpenChunkWriter(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20, Concurrency: 3, MinChunkSize: 5 << 20, MaxChunkSize: 5 << 30}, &gotRemote, &gotSize, &gotOptions)
	plaintext := testPlaintext(2*(5<<20) + (1 << 20) + 17)
	size := int64(len(plaintext))
	t1 := time.Date(2012, time.December, 17, 18, 32, 31, 0, time.UTC)
	src := object.NewStaticObjectInfo("dir/file.txt-WRONG-REMOTE", t1, size, true, nil, nil)

	info, w, err := f.OpenChunkWriter(ctx, "dir/file.txt", src)
	require.NoError(t, err)
	assert.Equal(t, f.cipher.EncryptFileName("dir/file.txt"), gotRemote)
	assert.Equal(t, f.cipher.EncryptedSize(size), gotSize)
	assert.Equal(t, int64(80*blockDataSize), info.ChunkSize)
	assert.Equal(t, 3, info.Concurrency)
	// the limits of the wrapped remote are on ciphertext and aren't passed on
	assert.Equal(t, int64(0), info.MinChunkSize)
	assert.Equal(t, int64(0), info.MaxChunkSize)

	// Write the chunks in parallel and out of order as a multi-thread copy does
	numChunks := int((size + info.ChunkSize - 1) / info.ChunkSize)
	var wg sync.WaitGroup
	for i := numChunks - 1; i >= 0; i-- {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			start := int64(i) * info.ChunkSize
			end := min(start+info.ChunkSize, size)
			n, err := w.WriteChunk(ctx, i, bytes.NewReader(plaintext[start:end]))
			assert.NoError(t, err)
			assert.Equal(t, end-start, n)
		}(i)
	}
	wg.Wait()
	require.NoError(t, w.Close(ctx))
	assert.True(t, mem.closed)
	assert.False(t, mem.aborted)
	assert.Len(t, mem.chunks, numChunks)

	// What the wrapped remote got is what the sequential encrypter writes
	// for the same nonce, which is all zeroes here
	var fileNonce nonce
	enc, err := f.cipher.newEncrypter(bytes.NewReader(plaintext), &fileNonce)
	require.NoError(t, err)
	want, err := io.ReadAll(enc)
	require.NoError(t, err)
	got := mem.bytes()
	assert.Equal(t, len(want), len(got))
	assert.True(t, bytes.Equal(want, got), "ciphertext differs from the sequential encrypter")

	// and it decrypts back to the plaintext
	dec, err := f.cipher.DecryptData(io.NopCloser(bytes.NewReader(got)))
	require.NoError(t, err)
	decrypted, err := io.ReadAll(dec)
	require.NoError(t, err)
	assert.True(t, bytes.Equal(plaintext, decrypted), "round trip differs")

	require.NoError(t, w.Abort(ctx))
	assert.True(t, mem.aborted)
}

// A chunk size asked for by the caller is plaintext and is passed on to
// the wrapped remote as the matching ciphertext size; the chunks follow
// what the wrapped remote settles on, which is the hint only if it took it.
func TestOpenChunkWriterChunkOption(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	src := object.NewStaticObjectInfo("file", time.Now(), 200*blockDataSize, true, nil, nil)

	// a remote which sizes its parts itself, 80 blocks worth, whatever the hint
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	info, _, err := f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 3*blockDataSize + 100})
	require.NoError(t, err)
	assert.Equal(t, int64(80*blockDataSize), info.ChunkSize)
	require.Len(t, gotOptions, 1)
	chunkOption, ok := gotOptions[0].(*fs.ChunkOption)
	require.True(t, ok)
	assert.Equal(t, encryptedChunkSize(3*blockDataSize, false), chunkOption.ChunkSize)
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 100 * blockDataSize})
	require.NoError(t, err)
	assert.Equal(t, int64(80*blockDataSize), info.ChunkSize)
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 5 << 30})
	require.NoError(t, err)
	assert.Equal(t, int64(80*blockDataSize), info.ChunkSize)

	// a remote which takes the hint - the chunks are exactly what was asked for
	f = newChunkTestFs(t, mem, fs.ChunkWriterInfo{}, &gotRemote, &gotSize, &gotOptions)
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 3*blockDataSize + 100})
	require.NoError(t, err)
	assert.Equal(t, int64(3*blockDataSize), info.ChunkSize)
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 100 * blockDataSize})
	require.NoError(t, err)
	assert.Equal(t, int64(100*blockDataSize), info.ChunkSize)
	// a hint the size of the biggest part S3 takes, from a remote with no known limit
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 5 << 30})
	require.NoError(t, err)
	assert.Equal(t, int64(5<<30), info.ChunkSize)

	// a remote which takes the hint but caps its parts - the ciphertext must stay under the cap
	f = newChunkTestFs(t, mem, fs.ChunkWriterInfo{MaxChunkSize: 5 << 30}, &gotRemote, &gotSize, &gotOptions)
	info, _, err = f.OpenChunkWriter(ctx, "file", src, &fs.ChunkOption{ChunkSize: 5 << 30})
	require.NoError(t, err)
	assert.Equal(t, chunkBlocks(5<<30, 5<<30)*blockDataSize, info.ChunkSize)
	assert.LessOrEqual(t, encryptedChunkSize(info.ChunkSize, true), int64(5<<30))
	assert.Greater(t, encryptedChunkSize(info.ChunkSize, false), int64(5<<30)-blockSize)
	assert.Equal(t, int64(0), info.MaxChunkSize)
}

// The chunks of a remote whose parts are as big as it takes are rounded
// down so the first chunk, header included, still fits its biggest part
func TestOpenChunkWriterMaxChunkSize(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	src := object.NewStaticObjectInfo("file", time.Now(), 200*blockDataSize, true, nil, nil)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 4000 << 20, MaxChunkSize: 4000 << 20}, &gotRemote, &gotSize, &gotOptions)
	info, _, err := f.OpenChunkWriter(ctx, "file", src)
	require.NoError(t, err)
	assert.Equal(t, int64(63984*blockDataSize), info.ChunkSize)
	assert.LessOrEqual(t, encryptedChunkSize(info.ChunkSize, true), int64(4000<<20))
	assert.Greater(t, encryptedChunkSize(info.ChunkSize+blockDataSize, true), int64(4000<<20))
}

// Chunks which can't be made to fit between the smallest and the biggest
// part of the wrapped remote are refused, and the upload the wrapped
// remote started is aborted
func TestOpenChunkWriterPartLimits(t *testing.T) {
	ctx := context.Background()
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	for _, test := range []struct {
		name string
		info fs.ChunkWriterInfo
		size int64
		ok   bool
	}{
		// rounded down to 79 blocks to fit the biggest part, which leaves
		// the parts after part 0 under the smallest part
		{"below min", fs.ChunkWriterInfo{ChunkSize: 5 << 20, MinChunkSize: 5 << 20, MaxChunkSize: 5<<20 + 32}, 200 * blockDataSize, false},
		// the same limits are fine for a file in one chunk, which is the last
		{"below min one chunk", fs.ChunkWriterInfo{ChunkSize: 5 << 20, MinChunkSize: 5 << 20, MaxChunkSize: 5<<20 + 32}, 10 * blockDataSize, true},
		// not even one block and the header fit
		{"max too small", fs.ChunkWriterInfo{ChunkSize: 100, MaxChunkSize: 100}, 10, false},
		// rounded up the parts are never below the size asked for
		{"rounded up", fs.ChunkWriterInfo{ChunkSize: 5 << 20, MinChunkSize: 5 << 20, MaxChunkSize: 5 << 30}, 200 * blockDataSize, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			mem := &memChunkWriter{}
			f := newChunkTestFs(t, mem, test.info, &gotRemote, &gotSize, &gotOptions)
			src := object.NewStaticObjectInfo("file", time.Now(), test.size, true, nil, nil)
			info, w, err := f.OpenChunkWriter(ctx, "file", src)
			if test.ok {
				require.NoError(t, err)
				assert.NotNil(t, w)
				assert.Greater(t, info.ChunkSize, int64(0))
				assert.False(t, mem.aborted)
				return
			}
			assert.ErrorContains(t, err, "can't fit encrypted chunks")
			assert.True(t, fserrors.IsNoRetryError(err), "refusal must not be retried")
			assert.Nil(t, w)
			assert.True(t, mem.aborted, "the upload of the wrapped remote must be aborted")
		})
	}
}

// A wrapped remote which wants a checksum for the object gets the hash of
// the ciphertext the chunks make up
func TestOpenChunkWriterHash(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	plaintext := testPlaintext(2*(5<<20) + 12345)
	src := uploadFile(t, makeTempLocalFs(t), "file.bin", string(plaintext))

	info, w, err := f.OpenChunkWriter(ctx, "file.bin", src)
	require.NoError(t, err)
	// chunks written one at a time, last first, as the generic tests do
	size := int64(len(plaintext))
	for start := (size - 1) / info.ChunkSize * info.ChunkSize; start >= 0; start -= info.ChunkSize {
		end := min(start+info.ChunkSize, size)
		_, err := w.WriteChunk(ctx, int(start/info.ChunkSize), bytes.NewReader(plaintext[start:end]))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close(ctx))

	require.NotNil(t, mem.src)
	gotHash, err := mem.src.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%x", md5.Sum(mem.bytes())), gotHash)

	// A source which isn't an object, uploaded under another name, has no
	// data to hash
	static := object.NewStaticObjectInfo("file.bin-WRONG-REMOTE", time.Now(), 100, true, nil, nil)
	_, _, err = f.OpenChunkWriter(ctx, "file.bin", static)
	require.NoError(t, err)
	gotHash, err = mem.src.Hash(ctx, hash.MD5)
	require.NoError(t, err)
	assert.Equal(t, "", gotHash)
}

// A chunk which doesn't match the data the checksum given to the wrapped
// remote was worked out from - the source changed - is refused before it
// is uploaded
func TestOpenChunkWriterSourceChanged(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	plaintext := testPlaintext(2*(5<<20) + 12345)
	src := uploadFile(t, makeTempLocalFs(t), "file.bin", string(plaintext))

	info, w, err := f.OpenChunkWriter(ctx, "file.bin", src)
	require.NoError(t, err)
	changed := bytes.Clone(plaintext)
	changed[info.ChunkSize+1] ^= 0xff
	_, err = w.WriteChunk(ctx, 0, bytes.NewReader(changed[:info.ChunkSize]))
	require.NoError(t, err)
	_, err = w.WriteChunk(ctx, 1, bytes.NewReader(changed[info.ChunkSize:2*info.ChunkSize]))
	assert.ErrorContains(t, err, "corrupted on transfer")
	assert.Len(t, mem.chunks, 1, "the changed chunk must not be uploaded")
}

// A whole encrypted chunk is buffered, so an upload whose chunks don't
// fit --max-buffer-memory is refused rather than resized
func TestOpenChunkWriterMaxBufferMemory(t *testing.T) {
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	src := object.NewStaticObjectInfo("file", time.Now(), 2000*blockDataSize, true, nil, nil)
	ctx, ci := fs.AddConfig(context.Background())
	limits := fs.ChunkWriterInfo{ChunkSize: 64 << 20, MinChunkSize: 5 << 20, MaxChunkSize: 5 << 30}

	// the encrypted chunk is 64 MiB + 16 KiB + 32 bytes: 65 pages
	for _, test := range []struct {
		limit fs.SizeSuffix
		ok    bool
	}{
		{65 << 20, true},
		{64 << 20, false},
		{512 << 10, false},
	} {
		ci.MaxBufferMemory = test.limit
		mem := &memChunkWriter{}
		f := newChunkTestFs(t, mem, limits, &gotRemote, &gotSize, &gotOptions)
		info, _, err := f.OpenChunkWriter(ctx, "file", src)
		if test.ok {
			require.NoError(t, err, "limit %v", test.limit)
			assert.Equal(t, int64(64<<20), info.ChunkSize)
			continue
		}
		assert.ErrorContains(t, err, "--max-buffer-memory", "limit %v", test.limit)
		assert.True(t, fserrors.IsNoRetryError(err), "limit %v", test.limit)
		assert.True(t, mem.aborted, "limit %v", test.limit)
	}
}

// The tags of the first pass which worked out the checksum are kept; a
// later pass over different data - the source changed between two
// requests for the checksum - is caught, and so are concurrent passes
func TestBlockTagRecord(t *testing.T) {
	var r blockTagRecord
	_, done, _ := r.get()
	assert.False(t, done)
	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.add([]uint64{1, 2, 3})
		}()
	}
	wg.Wait()
	tags, done, differ := r.get()
	assert.Equal(t, []uint64{1, 2, 3}, tags)
	assert.True(t, done)
	assert.False(t, differ)
	r.add([]uint64{1, 2, 4})
	tags, _, differ = r.get()
	assert.Equal(t, []uint64{1, 2, 3}, tags, "the first pass is kept")
	assert.True(t, differ)
}

// A source which changes between two requests for its checksum is
// refused when the chunk writer is opened
func TestOpenChunkWriterChecksumChanged(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	plaintext := testPlaintext(2*(5<<20) + 12345)
	src := uploadFile(t, makeTempLocalFs(t), "file.bin", string(plaintext))
	wrapped := f.Fs.Features().OpenChunkWriter
	f.Fs.Features().OpenChunkWriter = func(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
		info, w, err := wrapped(ctx, remote, src, options...)
		// a second pass for the checksum reads different data
		src.(*ObjectInfo).tags.add([]uint64{1, 2, 3})
		return info, w, err
	}
	_, _, err := f.OpenChunkWriter(ctx, "file.bin", src)
	assert.ErrorContains(t, err, "source changed")
	assert.True(t, mem.aborted)
}

// A chunked encrypted upload can't check its chunks without a size
func TestOpenChunkWriterUnknownSize(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	src := object.NewStaticObjectInfo("file", time.Now(), -1, true, nil, nil)
	_, _, err := f.OpenChunkWriter(ctx, "file", src)
	assert.ErrorContains(t, err, "unknown size")
	assert.Equal(t, "", gotRemote, "wrapped remote must not be opened")
}

// The chunk writer is only kept when the wrapped remote takes chunks of
// a size other than the one it asked for; the encrypting writer itself
// then needs its chunks exactly the size it asks for, and the copy is
// told not to buffer the plaintext
func TestRestrictChunkWriter(t *testing.T) {
	open := func(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
		return fs.ChunkWriterInfo{}, nil, nil
	}
	ft := &fs.Features{OpenChunkWriter: open}
	restrictChunkWriter(ft, false)
	assert.Nil(t, ft.OpenChunkWriter)
	assert.False(t, ft.ChunkWriterDoesntSeek)

	ft = &fs.Features{OpenChunkWriter: open, ChunkWriterVariableSize: true}
	restrictChunkWriter(ft, false)
	assert.NotNil(t, ft.OpenChunkWriter)
	assert.True(t, ft.ChunkWriterDoesntSeek)
	assert.False(t, ft.ChunkWriterVariableSize)

	// with the data unencrypted the wrapped remote's writer is used as is
	ft = &fs.Features{OpenChunkWriter: open, ChunkWriterVariableSize: true}
	restrictChunkWriter(ft, true)
	assert.NotNil(t, ft.OpenChunkWriter)
	assert.False(t, ft.ChunkWriterDoesntSeek)
	assert.True(t, ft.ChunkWriterVariableSize)
	ft = &fs.Features{OpenChunkWriter: open, ChunkWriterDoesntSeek: true}
	restrictChunkWriter(ft, true)
	assert.NotNil(t, ft.OpenChunkWriter)
	assert.True(t, ft.ChunkWriterDoesntSeek)
	assert.False(t, ft.ChunkWriterVariableSize)
}

// The chunk writer features of a crypt remote follow the wrapped remote
func TestNewFsChunkWriterFeatures(t *testing.T) {
	ctx := context.Background()
	ri, err := fs.Find("crypt")
	require.NoError(t, err)
	open := func(ctx context.Context, remote string, src fs.ObjectInfo, options ...fs.OpenOption) (fs.ChunkWriterInfo, fs.ChunkWriter, error) {
		return fs.ChunkWriterInfo{}, nil, nil
	}
	for i, test := range []struct {
		variableSize     bool
		doesntSeek       bool
		noDataEncryption bool
		wantWriter       bool
		wantVariableSize bool
		wantDoesntSeek   bool
	}{
		{variableSize: false, wantWriter: false},
		{variableSize: true, wantWriter: true, wantVariableSize: false, wantDoesntSeek: true},
		{variableSize: false, noDataEncryption: true, wantWriter: true},
		{variableSize: true, doesntSeek: true, noDataEncryption: true, wantWriter: true, wantVariableSize: true, wantDoesntSeek: true},
	} {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			name := fmt.Sprintf("mockcw%d", i)
			wrapped, err := mockfs.NewFs(ctx, name, "", nil)
			require.NoError(t, err)
			ft := wrapped.Features()
			ft.OpenChunkWriter = open
			ft.ChunkWriterVariableSize = test.variableSize
			ft.ChunkWriterDoesntSeek = test.doesntSeek
			cache.Put(name+":", wrapped)
			defer cache.ClearConfig(name)
			m := configmap.Simple{
				"remote":             name + ":",
				"password":           obscure.MustObscure("potato"),
				"no_data_encryption": fmt.Sprint(test.noDataEncryption),
			}
			f, err := NewFs(ctx, "crypt"+name, "", fs.ConfigMap(ri.Prefix, ri.Options, "", m))
			require.NoError(t, err)
			got := f.Features()
			assert.Equal(t, test.wantWriter, got.OpenChunkWriter != nil, "OpenChunkWriter")
			assert.Equal(t, test.wantVariableSize, got.ChunkWriterVariableSize, "ChunkWriterVariableSize")
			assert.Equal(t, test.wantDoesntSeek, got.ChunkWriterDoesntSeek, "ChunkWriterDoesntSeek")
		})
	}
}

// With data encryption off the wrapped remote's chunk writer is used as is
func TestOpenChunkWriterNoDataEncryption(t *testing.T) {
	ctx := context.Background()
	mem := &memChunkWriter{}
	var (
		gotRemote  string
		gotSize    int64
		gotOptions []fs.OpenOption
	)
	f := newChunkTestFs(t, mem, fs.ChunkWriterInfo{ChunkSize: 5 << 20}, &gotRemote, &gotSize, &gotOptions)
	f.opt.NoDataEncryption = true
	src := object.NewStaticObjectInfo("file", time.Now(), 12345, true, nil, nil)
	info, w, err := f.OpenChunkWriter(ctx, "file", src)
	require.NoError(t, err)
	assert.Equal(t, int64(5<<20), info.ChunkSize)
	assert.Equal(t, int64(12345), gotSize)
	assert.Equal(t, f.cipher.EncryptFileName("file"), gotRemote)
	assert.Same(t, mem, w)
}

// A chunk which is not the size the file says it should be must not reach
// the wrapped remote - it would leave a short block inside the file.
func TestChunkWriterShortChunk(t *testing.T) {
	ctx := context.Background()
	c, err := newCipher(NameEncryptionStandard, "", "", true, nil)
	require.NoError(t, err)
	mem := &memChunkWriter{}
	plaintext := testPlaintext(5 * blockDataSize)
	cw := &chunkWriter{
		f:         &Fs{cipher: c},
		w:         mem,
		chunkSize: 2 * blockDataSize,
		size:      int64(len(plaintext)),
	}
	// one block short
	_, err = cw.WriteChunk(ctx, 0, bytes.NewReader(plaintext[:blockDataSize]))
	assert.ErrorContains(t, err, "expected 131072 bytes of plaintext but read 65536")
	// one block too long
	_, err = cw.WriteChunk(ctx, 0, bytes.NewReader(plaintext[:3*blockDataSize]))
	assert.ErrorContains(t, err, "expected 131072 bytes of plaintext but read 196608")
	// beyond the end of the file
	_, err = cw.WriteChunk(ctx, 3, bytes.NewReader(nil))
	assert.ErrorContains(t, err, "beyond the end of the file")
	_, err = cw.WriteChunk(ctx, -1, bytes.NewReader(nil))
	assert.ErrorContains(t, err, "invalid chunk number")
	assert.Empty(t, mem.chunks)
	// the last chunk is allowed to be short
	n, err := cw.WriteChunk(ctx, 2, bytes.NewReader(plaintext[4*blockDataSize:]))
	require.NoError(t, err)
	assert.Equal(t, int64(blockDataSize), n)
	assert.Len(t, mem.chunks, 1)
	// a chunk starting exactly at the end of a file is an error, not an empty part
	cw.size = 4 * blockDataSize
	_, err = cw.WriteChunk(ctx, 2, bytes.NewReader(nil))
	assert.ErrorContains(t, err, "beyond the end of the file")
	assert.Len(t, mem.chunks, 1)
	// except for an empty file, whose only chunk is the header
	mem = &memChunkWriter{}
	cw.w = mem
	cw.size = 0
	n, err = cw.WriteChunk(ctx, 0, bytes.NewReader(nil))
	require.NoError(t, err)
	assert.Equal(t, int64(0), n)
	require.Len(t, mem.chunks, 1)
	assert.Equal(t, fileHeaderSize, len(mem.chunks[0]))
}

// InternalTest is called by fstests.Run to extra tests
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("ObjectInfo", func(t *testing.T) { testObjectInfo(t, f, false) })
	t.Run("ObjectInfoWrap", func(t *testing.T) { testObjectInfo(t, f, true) })
	t.Run("ComputeHash", func(t *testing.T) { testComputeHash(t, f) })
}
