package smb

import (
	"context"
	"fmt"
	"os"

	"github.com/cloudsoda/go-smb2"
	"golang.org/x/sync/errgroup"
)

type smbChunkWriterFile struct {
	*smb2.File
	c *conn
}

func (w *smbChunkWriter) getFile(ctx context.Context) (f *smbChunkWriterFile, err error) {
	w.poolMu.Lock()
	if len(w.pool) > 0 {
		f = w.pool[0]
		w.pool = w.pool[1:]
	}
	w.poolMu.Unlock()

	if f != nil {
		return f, nil
	}

	w.f.addSession() // Show session in use

	c, err := w.f.getConnection(ctx, w.share)
	if err != nil {
		w.f.removeSession()
		return nil, err
	}

	fl, err := c.smbShare.OpenFile(w.filename, os.O_WRONLY, 0o644)
	if err != nil {
		w.f.putConnection(&c, err)
		w.f.removeSession()
		return nil, fmt.Errorf("failed to open: %w", err)
	}

	return &smbChunkWriterFile{File: fl, c: c}, nil
}

func (w *smbChunkWriter) putFile(pf **smbChunkWriterFile, err error) {
	if pf == nil {
		return
	}
	f := *pf
	if f == nil {
		return
	}
	*pf = nil

	if err != nil {
		_ = f.Close()
		w.f.putConnection(&f.c, err)
		w.f.removeSession()
		return
	}

	w.poolMu.Lock()
	w.pool = append(w.pool, f)
	w.poolMu.Unlock()
}

func (w *smbChunkWriter) drainPool(ctx context.Context) (err error) {
	w.poolMu.Lock()
	defer w.poolMu.Unlock()

	if len(w.pool) == 0 {
		return nil
	}

	g, _ := errgroup.WithContext(ctx)
	for i, f := range w.pool {
		g.Go(func() error {
			err := f.Close()
			w.f.putConnection(&f.c, err)
			w.f.removeSession()
			w.pool[i] = nil
			return err
		})
	}
	err = g.Wait()
	w.pool = nil

	return err
}
