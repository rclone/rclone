package xpan

import (
	"context"
	"crypto/md5"
	"fmt"
	"hash"
	"io"
	"net/http"
	"net/url"
	"strconv"

	"github.com/rclone/rclone/lib/rest"
)

// downloadReader remote file reader
type downloadReader struct {
	fileSize int64
	dlink    string
	ctx      context.Context
	fs       *Fs

	// internal
	_chunkReaderCloser io.ReadCloser
	_chunkReadEnd      bool
	_fileReadEnd       bool
	_chunkCounter      int
}

func (r *downloadReader) Read(p []byte) (n int, err error) {
	if r._chunkReaderCloser == nil || r._chunkReadEnd {
		if r._fileReadEnd {
			return 0, io.EOF
		}
		err = r.readChunck()
		if err != nil {
			return
		}
	}
	n, err = r._chunkReaderCloser.Read(p)
	if err == io.EOF {
		if r._fileReadEnd {
			return
		}
		r._chunkReadEnd = true
		err = r._chunkReaderCloser.Close()
	}
	return
}

func (r *downloadReader) readChunck() error {
	dlink, err := url.Parse(r.dlink)
	if err != nil {
		return err
	}
	token, err := r.fs.ts.Token()
	if err != nil {
		return err
	}
	params := dlink.Query()
	params.Set("access_token", token.AccessToken)
	limit := 50 * 1024 * 1024
	opts := rest.Opts{
		Method:       "GET",
		Path:         dlink.Path,
		RootURL:      fmt.Sprintf("%s://%s", dlink.Scheme, dlink.Host),
		Parameters:   params,
		ContentRange: fmt.Sprintf("bytes=%d-%d", r._chunkCounter*limit, (r._chunkCounter+1)*limit-1),
		ExtraHeaders: map[string]string{
			"User-Agent": "pan.baidu.com",
		},
	}
	var resp *http.Response
	err = r.fs.pacer.Call(func() (bool, error) {
		resp, err = r.fs.srv.Call(r.ctx, &opts)
		return false, err
	})
	if err != nil {
		return err
	}
	r._chunkReaderCloser = resp.Body
	r._chunkReadEnd = false
	cl := resp.Header.Get("Content-Length")
	clen, _ := strconv.ParseInt(cl, 10, 64)
	r._fileReadEnd = int64(r._chunkCounter*limit+int(clen)) == r.fileSize
	r._chunkCounter++
	return nil
}

// chunckReader sum md5 hash of each chunk
type chunckReader struct {
	in        io.Reader
	chunkSize int

	// internal
	_wrapReader        io.Reader
	_md5Hash           hash.Hash
	_md5s              []string // chuncks md5s
	_chunkBytesCounter int      // latest chunk bytes
}

func (r *chunckReader) Read(p []byte) (n int, err error) {
	if r._wrapReader == nil {
		r._md5Hash = md5.New()
		r._wrapReader = io.TeeReader(r.in, r._md5Hash)
	}

	c := r.chunkSize - r._chunkBytesCounter
	if len(p) > c {
		p = p[:c]
	}

	n, err = r._wrapReader.Read(p)
	r._chunkBytesCounter += n

	// file EOF
	if err == io.EOF {
		b := r._md5Hash.Sum(nil)
		r._md5s = append(r._md5s, fmt.Sprintf("%x", b))
		return
	}

	// chunk EOF
	if r._chunkBytesCounter == r.chunkSize {
		b := r._md5Hash.Sum(nil)
		r._md5s = append(r._md5s, fmt.Sprintf("%x", b))
		// for next chunk
		r._md5Hash = md5.New()
		r._wrapReader = io.TeeReader(r.in, r._md5Hash)
		r._chunkBytesCounter = 0
	}
	return
}

func newDownloadReader(ctx context.Context, fs *Fs, fileSize int64, dlink string) *downloadReader {
	return &downloadReader{
		fs:       fs,
		fileSize: fileSize,
		dlink:    dlink,
		ctx:      ctx,
	}
}

func newChunkReader(in io.Reader, chunkSize int) *chunckReader {
	return &chunckReader{in: in, chunkSize: chunkSize}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
