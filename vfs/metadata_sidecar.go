package vfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfsmeta"
)

type sidecarStore struct {
	vfs *VFS
	ext string
}

func newSidecarStore(vfs *VFS, ext string) *sidecarStore {
	return &sidecarStore{vfs: vfs, ext: ext}
}

func (s *sidecarStore) name(p string) string {
	return strings.TrimSuffix(p, "/") + s.ext
}

func (s *sidecarStore) Load(ctx context.Context, p string, isDir bool) (vfsmeta.Meta, error) {
	b, err := s.vfs.ReadFile(s.name(p))
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	meta, err := decodeSidecar(b)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	return meta, nil
}

func (s *sidecarStore) Save(ctx context.Context, p string, isDir bool, m vfsmeta.Meta) error {
	cur, err := s.Load(ctx, p, isDir)
	if err != nil {
		if !errors.Is(err, fs.ErrorObjectNotFound) && !errors.Is(err, ENOENT) {
			return err
		}
		cur = vfsmeta.Meta{}
	}
	cur.Merge(m)

	b, err := encodeSidecar(cur)
	if err != nil {
		return err
	}
	name := s.name(p)
	return s.writeWithFallback(name, b)
}

func (s *sidecarStore) Rename(ctx context.Context, oldPath, newPath string, isDir bool) error {
	return s.vfs.Rename(s.name(oldPath), s.name(newPath))
}

func (s *sidecarStore) Delete(ctx context.Context, p string, isDir bool) error {
	name := s.name(p)
	if err := s.vfs.Remove(name); err != nil {
		if errors.Is(err, ENOENT) {
			return nil
		}
		return err
	}
	return nil
}

func (s *sidecarStore) renameSidecar(oldName, newName string) error {
	oldDir, oldLeaf, err := s.vfs.StatParent(oldName)
	if err != nil {
		return err
	}
	newDir, newLeaf, err := s.vfs.StatParent(newName)
	if err != nil {
		return err
	}
	return oldDir.Rename(oldLeaf, newLeaf, newDir)
}

func (s *sidecarStore) writeAtomic(name string, data []byte) error {
	tmp := name + ".tmp"
	if err := s.vfs.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := s.renameSidecar(tmp, name); err != nil {
		_ = s.vfs.Remove(tmp)
		return fmt.Errorf("rename sidecar metadata failed: %w", err)
	}
	return nil
}

func (s *sidecarStore) writeWithFallback(name string, data []byte) error {
	if err := s.writeAtomic(name, data); err == nil {
		return nil
	} else if err != nil {
		if err2 := s.vfs.WriteFile(name, data, 0o600); err2 != nil {
			return fmt.Errorf("sidecar write fallback failed: %v (original error: %w)", err2, err)
		}
	}
	return nil
}

func encodeSidecar(meta vfsmeta.Meta) ([]byte, error) {
	type diskMeta struct {
		Mode  *string `json:"mode,omitempty"`
		UID   *string `json:"uid,omitempty"`
		GID   *string `json:"gid,omitempty"`
		Mtime *string `json:"mtime,omitempty"`
		Atime *string `json:"atime,omitempty"`
		Btime *string `json:"btime,omitempty"`
	}
	toUint := func(u *uint32) *string {
		if u == nil {
			return nil
		}
		s := strconv.FormatUint(uint64(*u), 10)
		return &s
	}
	toTime := func(t *time.Time) *string {
		if t == nil {
			return nil
		}
		s := t.UTC().Format(time.RFC3339Nano)
		return &s
	}
	d := diskMeta{
		Mode:  toUint(meta.Mode),
		UID:   toUint(meta.UID),
		GID:   toUint(meta.GID),
		Mtime: toTime(meta.Mtime),
		Atime: toTime(meta.Atime),
		Btime: toTime(meta.Btime),
	}
	return json.Marshal(d)
}

func decodeSidecar(data []byte) (vfsmeta.Meta, error) {
	type diskMeta struct {
		Mode  *string `json:"mode,omitempty"`
		UID   *string `json:"uid,omitempty"`
		GID   *string `json:"gid,omitempty"`
		Mtime *string `json:"mtime,omitempty"`
		Atime *string `json:"atime,omitempty"`
		Btime *string `json:"btime,omitempty"`
	}
	var d diskMeta
	if err := json.Unmarshal(data, &d); err != nil {
		return vfsmeta.Meta{}, err
	}
	parseUint := func(s *string) (*uint32, error) {
		if s == nil || *s == "" {
			return nil, nil
		}
		val, err := strconv.ParseUint(*s, 10, 32)
		if err != nil {
			return nil, err
		}
		u := uint32(val)
		return &u, nil
	}
	parseTime := func(s *string) (*time.Time, error) {
		if s == nil || *s == "" {
			return nil, nil
		}
		t, err := time.Parse(time.RFC3339Nano, *s)
		if err != nil {
			return nil, err
		}
		t = t.UTC()
		return &t, nil
	}
	mode, err := parseUint(d.Mode)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	uid, err := parseUint(d.UID)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	gid, err := parseUint(d.GID)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	mtime, err := parseTime(d.Mtime)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	atime, err := parseTime(d.Atime)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	btime, err := parseTime(d.Btime)
	if err != nil {
		return vfsmeta.Meta{}, err
	}
	return vfsmeta.Meta{
		Mode:  mode,
		UID:   uid,
		GID:   gid,
		Mtime: mtime,
		Atime: atime,
		Btime: btime,
	}, nil
}
