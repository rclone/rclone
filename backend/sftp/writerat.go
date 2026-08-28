//go:build !plan9

package sftp

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"

	"github.com/pkg/sftp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/filepool"
)

// poolFile is a pooled write handle together with the connection it lives on.
type poolFile struct {
	file *sftp.File
	c    *conn
}

// openPoolFile opens a fresh write handle on its own connection for path. The
// file must already exist: the handle opens O_WRONLY only, so it never races
// another handle to create or truncate.
func (f *Fs) openPoolFile(path string) func(context.Context) (*poolFile, error) {
	return func(ctx context.Context) (*poolFile, error) {
		c, err := f.getSftpConnection(ctx)
		if err != nil {
			return nil, err
		}
		file, err := c.sftpClient.OpenFile(path, os.O_WRONLY)
		if err != nil {
			f.putSftpConnection(&c, err)
			return nil, err
		}
		return &poolFile{file: file, c: c}, nil
	}
}

// releasePoolFile closes a pooled handle and returns its connection.
func (f *Fs) releasePoolFile(pf *poolFile, err error) error {
	closeErr := pf.file.Close()
	if err == nil {
		err = closeErr
	}
	f.putSftpConnection(&pf.c, err)
	return closeErr
}

// sftpWriterAt is the fs.WriterAtCloser used by the core's multi-thread copy.
// WriteAt is called concurrently at non-overlapping offsets, each borrowing its
// own handle from the pool.
type sftpWriterAt struct {
	fs      *Fs
	pool    *filepool.Pool[*poolFile]
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

	pf, err := w.pool.Get()
	if err != nil {
		return 0, err
	}
	n, writeErr := pf.file.WriteAt(p, off)
	w.pool.Put(pf, writeErr)
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
	err := w.pool.Drain()
	w.fs.removeSession()
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
			return nil, fmt.Errorf("OpenWriterAt: truncate failed (the server may not support multi-thread uploads; try without --sftp-multithread-upload): %w", truncErr)
		}
	}
	if closeErr := file.Close(); closeErr != nil {
		f.putSftpConnection(&c, closeErr)
		return nil, fmt.Errorf("OpenWriterAt: close failed: %w", closeErr)
	}
	f.putSftpConnection(&c, nil)

	f.addSession()
	return &sftpWriterAt{fs: f, pool: filepool.New(ctx, f.openPoolFile(path), f.releasePoolFile)}, nil
}
