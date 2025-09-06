package vfs

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfsmeta"
)

type backendStore struct {
	vfs *VFS
}

func newBackendStore(vfs *VFS) *backendStore {
	return &backendStore{vfs: vfs}
}

func (s *backendStore) Load(ctx context.Context, p string, isDir bool) (vfsmeta.Meta, error) {
	entry, err := s.entry(ctx, p)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	md, err := fs.GetMetadata(ctx, entry)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	var m vfsmeta.Meta
	if v, ok := md["mode"]; ok {
		if n, err := strconv.ParseUint(v, 8, 32); err == nil {
			u := uint32(n)
			m.Mode = &u
		}
	}
	if v, ok := md["uid"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			u := uint32(n)
			m.UID = &u
		}
	}
	if v, ok := md["gid"]; ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			u := uint32(n)
			m.GID = &u
		}
	}
	if v, ok := md["mtime"]; ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			m.Mtime = &t
		}
	}
	if v, ok := md["atime"]; ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			m.Atime = &t
		}
	}
	if v, ok := md["btime"]; ok {
		if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
			m.Btime = &t
		}
	}
	return m, nil
}

func (s *backendStore) Save(ctx context.Context, p string, isDir bool, m vfsmeta.Meta) error {
	entry, err := s.entry(ctx, p)
	if err != nil {
		return err
	}
	md, _ := fs.GetMetadata(ctx, entry)
	if md == nil {
		md = fs.Metadata{}
	}
	if m.Mode != nil {
		md["mode"] = fmt.Sprintf("%o", *m.Mode)
	}
	if m.UID != nil {
		md["uid"] = fmt.Sprintf("%d", *m.UID)
	}
	if m.GID != nil {
		md["gid"] = fmt.Sprintf("%d", *m.GID)
	}
	if m.Mtime != nil {
		md["mtime"] = m.Mtime.Format(time.RFC3339Nano)
	}
	if m.Atime != nil {
		md["atime"] = m.Atime.Format(time.RFC3339Nano)
	}
	if m.Btime != nil {
		md["btime"] = m.Btime.Format(time.RFC3339Nano)
	}
	w, ok := entry.(fs.SetMetadataer)
	if !ok {
		return fs.ErrorNotImplemented
	}
	return w.SetMetadata(ctx, md)
}

func (s *backendStore) Rename(ctx context.Context, oldPath, newPath string, isDir bool) error {
	return nil
}

func (s *backendStore) Delete(ctx context.Context, p string, isDir bool) error {
	return nil
}

func (s *backendStore) entry(ctx context.Context, p string) (fs.DirEntry, error) {
	node, err := s.vfs.Stat(p)
	if err != nil {
		return nil, err
	}
	e := node.DirEntry()
	if e == nil {
		return nil, fs.ErrorObjectNotFound
	}
	return e, nil
}
