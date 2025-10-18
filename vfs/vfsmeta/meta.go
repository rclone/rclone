// Package vfsmeta defines metadata structures and store interfaces
// used by the VFS for persisting POSIX-like attributes.
package vfsmeta

import (
	"context"
	"time"
)

// Meta holds optional POSIX-like attributes.
type Meta struct {
	Mode  *uint32    `json:"mode,omitempty"`
	UID   *uint32    `json:"uid,omitempty"`
	GID   *uint32    `json:"gid,omitempty"`
	Mtime *time.Time `json:"mtime,omitempty"`
	Atime *time.Time `json:"atime,omitempty"`
	Btime *time.Time `json:"btime,omitempty"`
}

// Merge overlays non-nil fields from d into m.
func (m *Meta) Merge(d Meta) {
	if d.Mode != nil {
		m.Mode = d.Mode
	}
	if d.UID != nil {
		m.UID = d.UID
	}
	if d.GID != nil {
		m.GID = d.GID
	}
	if d.Mtime != nil {
		m.Mtime = d.Mtime
	}
	if d.Atime != nil {
		m.Atime = d.Atime
	}
	if d.Btime != nil {
		m.Btime = d.Btime
	}
}

// Store defines a metadata persistence backend.
type Store interface {
	Load(ctx context.Context, path string, isDir bool) (Meta, error)
	Save(ctx context.Context, path string, isDir bool, m Meta) error
	Rename(ctx context.Context, oldPath, newPath string, isDir bool) error
	Delete(ctx context.Context, path string, isDir bool) error
}
