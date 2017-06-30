package fs

import "time"

// NewStaticObjectInfo returns a static ObjectInfo
// If hashes is nil and fs is not nil, the hash map will be replaced with
// empty hashes of the types supported by the fs.
func NewStaticObjectInfo(remote string, modTime time.Time, size int64, storable bool, hashes map[HashType]string, fs Info) ObjectInfo {
	info := &staticObjectInfo{
		remote:   remote,
		modTime:  modTime,
		size:     size,
		storable: storable,
		hashes:   hashes,
		fs:       fs,
	}
	if fs != nil && hashes == nil {
		set := fs.Hashes().Array()
		info.hashes = make(map[HashType]string)
		for _, ht := range set {
			info.hashes[ht] = ""
		}
	}
	return info
}

type staticObjectInfo struct {
	remote   string
	modTime  time.Time
	size     int64
	storable bool
	hashes   map[HashType]string
	fs       Info
}

func (i *staticObjectInfo) Fs() Info           { return i.fs }
func (i *staticObjectInfo) Remote() string     { return i.remote }
func (i *staticObjectInfo) String() string     { return i.remote }
func (i *staticObjectInfo) ModTime() time.Time { return i.modTime }
func (i *staticObjectInfo) Size() int64        { return i.size }
func (i *staticObjectInfo) Storable() bool     { return i.storable }
func (i *staticObjectInfo) Hash(h HashType) (string, error) {
	if len(i.hashes) == 0 {
		return "", ErrHashUnsupported
	}
	if hash, ok := i.hashes[h]; ok {
		return hash, nil
	}
	return "", ErrHashUnsupported
}
