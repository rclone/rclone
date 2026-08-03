package sftp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// poolFile is an open write handle together with the connection it lives on.
type poolFile struct {
	file *sftp.File
	c    *conn
}

// filePool hands out write handles to a single remote path, one per connection,
// so concurrent WriteAt calls each get an independent handle.
type filePool struct {
	ctx  context.Context
	fs   *Fs
	path string

	mu   sync.Mutex
	pool []*poolFile
}

func newFilePool(ctx context.Context, f *Fs, path string) *filePool {
	return &filePool{ctx: ctx, fs: f, path: path}
}

// get returns a free handle, opening a new one on a new connection if none are
// free. The file must already exist: the handle opens O_WRONLY only, so it never
// races another handle to create or truncate.
func (p *filePool) get() (*poolFile, error) {
	p.mu.Lock()
	if n := len(p.pool); n > 0 {
		pf := p.pool[n-1]
		p.pool = p.pool[:n-1]
		p.mu.Unlock()
		return pf, nil
	}
	p.mu.Unlock()

	c, err := p.fs.getSftpConnection(p.ctx)
	if err != nil {
		return nil, err
	}
	file, err := c.sftpClient.OpenFile(p.path, os.O_WRONLY)
	if err != nil {
		p.fs.putSftpConnection(&c, err)
		return nil, err
	}
	return &poolFile{file: file, c: c}, nil
}

// put returns a handle to the pool, or tears it down if the write that used it
// failed and the connection may be in an unknown state.
func (p *filePool) put(pf *poolFile, err error) {
	if pf == nil {
		return
	}
	if err != nil {
		_ = pf.file.Close()
		p.fs.putSftpConnection(&pf.c, err)
		return
	}
	p.mu.Lock()
	p.pool = append(p.pool, pf)
	p.mu.Unlock()
}

// drain closes every pooled handle and returns its connection.
func (p *filePool) drain() error {
	p.mu.Lock()
	files := p.pool
	p.pool = nil
	p.mu.Unlock()

	g, _ := errgroup.WithContext(p.ctx)
	for _, pf := range files {
		g.Go(func() error {
			err := pf.file.Close()
			p.fs.putSftpConnection(&pf.c, err)
			return err
		})
	}
	return g.Wait()
}

// sftpWriterAt is the fs.WriterAtCloser used by the core's multi-thread copy.
// WriteAt is called concurrently at non-overlapping offsets, each borrowing its
// own handle from the pool.
type sftpWriterAt struct {
	pool    *filePool
	closeMu sync.Mutex
	closed  bool
	wg      sync.WaitGroup
}

// WriteAt writes p at offset off using a handle borrowed from the pool.
func (w *sftpWriterAt) WriteAt(p []byte, off int64) (int, error) {
	w.closeMu.Lock()
	if w.closed {
		w.closeMu.Unlock()
		return 0, errors.New("sftp: WriteAt on closed writer")
	}
	w.wg.Add(1)
	w.closeMu.Unlock()
	defer w.wg.Done()

	pf, err := w.pool.get()
	if err != nil {
		return 0, err
	}
	n, writeErr := pf.file.WriteAt(p, off)
	w.pool.put(pf, writeErr)
	if writeErr != nil {
		return n, fmt.Errorf("failed to write at offset %d: %w", off, writeErr)
	}
	return n, nil
}

// Close waits for outstanding writes then closes every pooled handle.
func (w *sftpWriterAt) Close() error {
	w.closeMu.Lock()
	if w.closed {
		w.closeMu.Unlock()
		return nil
	}
	w.closed = true
	w.closeMu.Unlock()

	w.wg.Wait()
	err := w.pool.drain()
	w.pool.fs.removeSession()
	return err
}

// OpenWriterAt opens remote for random-access writes, truncating any existing
// object, and pre-sizes it to size (if known) so every chunk offset is valid.
//
// The file is created and truncated once here, on a single connection, so the
// pooled handles open O_WRONLY and never race to truncate each other's data.
func (f *Fs) OpenWriterAt(ctx context.Context, remote string, size int64) (fs.WriterAtCloser, error) {
	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: %w", err)
	}
	path := f.remotePath(remote)

	c, err := f.getSftpConnection(ctx)
	if err != nil {
		return nil, fmt.Errorf("OpenWriterAt: %w", err)
	}
	file, err := c.sftpClient.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC)
	if err != nil {
		f.putSftpConnection(&c, err)
		return nil, fmt.Errorf("OpenWriterAt: create failed: %w", err)
	}
	if size > 0 {
		if truncErr := file.Truncate(size); truncErr != nil {
			_ = file.Close()
			f.putSftpConnection(&c, truncErr)
			return nil, fmt.Errorf("OpenWriterAt: truncate failed: %w", truncErr)
		}
	}
	if closeErr := file.Close(); closeErr != nil {
		f.putSftpConnection(&c, closeErr)
		return nil, fmt.Errorf("OpenWriterAt: close failed: %w", closeErr)
	}
	f.putSftpConnection(&c, nil)

	f.addSession()
	return &sftpWriterAt{pool: newFilePool(ctx, f, path)}, nil
}
