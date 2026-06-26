package smb

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/cloudsoda/go-smb2"
	"github.com/rclone/rclone/fs"
	"golang.org/x/sync/errgroup"
)

// FsInterface defines the methods that filePool needs from Fs
type FsInterface interface {
	getConnection(ctx context.Context, share string) (*conn, error)
	putConnection(pc **conn, err error)
	removeSession()
	isClosed(c *conn) bool
}

type file struct {
	*smb2.File
	c *conn
}

type filePool struct {
	ctx   context.Context
	fs    FsInterface
	share string
	path  string

	mu   sync.Mutex
	pool []*file
}

func newFilePool(ctx context.Context, fs FsInterface, share, path string) *filePool {
	return &filePool{
		ctx:   ctx,
		fs:    fs,
		share: share,
		path:  path,
	}
}

func (p *filePool) get() (*file, error) {
	p.mu.Lock()
	if len(p.pool) > 0 {
		// FIFO order: take the oldest connection so that all pooled
		// connections get used in rotation and none sit idle long
		// enough for the TCP deadline (--timeout) to expire.
		f := p.pool[0]
		p.pool = p.pool[1:]
		p.mu.Unlock()
		return f, nil
	}
	p.mu.Unlock()

	c, err := p.fs.getConnection(p.ctx, p.share)
	if err != nil {
		return nil, err
	}

	fl, err := c.smbShare.OpenFile(p.path, os.O_WRONLY, 0o644)
	if err != nil {
		p.fs.putConnection(&c, err)
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	return &file{File: fl, c: c}, nil
}

func (p *filePool) put(f *file, err error) {
	if f == nil {
		return
	}

	if err != nil {
		_ = f.Close()
		p.fs.putConnection(&f.c, err)
		return
	}

	p.mu.Lock()
	p.pool = append(p.pool, f)
	p.mu.Unlock()
}

func (p *filePool) drain() error {
	p.mu.Lock()
	files := p.pool
	p.pool = nil
	p.mu.Unlock()

	g, _ := errgroup.WithContext(p.ctx)
	for _, f := range files {
		g.Go(func() error {
			// During long multi-thread transfers, connections that
			// finished their last chunk early sit idle in the pool.
			// Their TCP deadline (--timeout) can expire, killing the
			// connection. When we try to Close the SMB file handle
			// over a dead connection it fails with "i/o timeout",
			// even though all data was written successfully.
			//
			// If the connection is dead, skip the Close — the server
			// will clean up the file handle when the TCP session ends.
			if p.fs.isClosed(f.c) {
				fs.Debugf(nil, "Skipping close on dead connection for %s", p.path)
				return nil
			}
			err := f.Close()
			p.fs.putConnection(&f.c, err)
			return err
		})
	}
	return g.Wait()
}
