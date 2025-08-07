package smb

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/cloudsoda/go-smb2"
	"golang.org/x/sync/errgroup"
)

// FsInterface defines the methods that filePool needs from Fs
type FsInterface interface {
	getConnection(ctx context.Context, share string) (*conn, error)
	putConnection(pc **conn, err error)
	removeSession()
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
		f := p.pool[len(p.pool)-1]
		p.pool = p.pool[:len(p.pool)-1]
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
			err := f.Close()
			p.fs.putConnection(&f.c, err)
			return err
		})
	}
	return g.Wait()
}
