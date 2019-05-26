// Code generated by vfsgen; DO NOT EDIT.

// +build !dev

package data

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	pathpkg "path"
	"time"
)

// Assets statically implements the virtual filesystem provided to vfsgen.
var Assets = func() http.FileSystem {
	fs := vfsgen۰FS{
		"/": &vfsgen۰DirInfo{
			name:    "/",
			modTime: time.Date(2019, 5, 26, 17, 10, 17, 848315327, time.UTC),
		},
		"/ConnectionManager.xml": &vfsgen۰CompressedFileInfo{
			name:             "ConnectionManager.xml",
			modTime:          time.Date(2019, 5, 26, 17, 11, 17, 180247092, time.UTC),
			uncompressedSize: 5505,

			compressedContent: []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xd4\x57\x4d\x6f\xdb\x3c\x0c\x3e\x3b\xbf\x22\xf0\x3d\xaf\x5b\xe0\x3d\x0c\x85\xe2\xa2\x4b\x3f\x10\x6c\x45\x83\x36\x0d\xb0\x53\xa1\xc9\x6c\xaa\xd5\xa6\x0c\x89\xee\xc7\xbf\x1f\x62\xc7\xb5\xdd\x38\x8b\xe3\xc8\x59\x76\x93\x68\x91\xcf\x63\x92\x22\x29\x76\xfa\x16\x85\xfd\x17\xd0\x46\x2a\x1c\xba\xc7\xff\x1d\xb9\x7d\x40\xa1\x02\x89\xf3\xa1\x7b\x3f\xbd\x1c\x7c\x71\x4f\xfd\x1e\x33\x22\x0e\xfa\x6f\x51\x88\x66\xe8\x26\x1a\x4f\x8c\x78\x82\x88\x9b\x41\x12\x63\x3c\x50\x7a\x7e\x62\x40\xbf\x48\x01\x83\xe3\xc1\x91\xeb\xf7\x1c\x66\x62\x10\xb3\xcc\xac\xdf\x73\x1c\x16\xf1\x5f\x4a\xfb\xc7\xcc\xcb\x16\xa9\x48\xa2\xd2\xfe\x11\xf3\xb2\x45\xcf\x61\x5e\x55\x8b\x71\x41\x52\xe1\x77\x69\x28\x55\xc8\xb6\x8b\xa5\xc3\x90\x47\xe0\x5f\x01\x4d\xb4\x22\x25\x54\x38\xc6\x47\xc5\xbc\x54\x9a\x7e\xe7\x7a\x9e\x44\x80\x94\x2b\x97\x44\xd9\x76\x69\xe2\x4e\x25\x5a\x40\x49\xd3\x71\x58\x20\x35\x64\x50\x2a\x21\xe6\x15\xdb\xe5\x77\x0d\x21\x27\x08\xee\x88\x13\xcc\xb8\x96\xfc\x67\x98\x1b\xaa\xd2\xa9\x3d\x98\x91\xf1\xaa\x6c\xd6\x90\x93\xf8\x6c\x83\x9a\xc4\xe7\x96\xc4\x8a\xed\x47\x14\xbc\x22\x0c\xab\x11\x99\x68\x88\xb9\x86\x4b\xa5\x47\x0a\x31\xe3\xd6\x26\x2c\xb7\x10\x29\x82\x35\xc1\xad\xf8\x41\x62\x53\x37\x9c\x3d\x9c\xdd\x5e\x3d\x4c\x7f\x4c\x2e\x1e\xec\x86\x69\x02\x50\xfa\xdd\x6b\x8e\x7c\x0e\xda\x2a\xdf\x1a\xeb\x76\x49\x8f\xcf\x3b\xe2\xbb\x30\xbc\x2b\xd5\xf3\x1c\xde\x2a\xc7\x92\xd5\x5d\x09\x36\xf1\xe3\x16\xf7\xb5\x33\x47\x9e\xcd\xa6\x9a\xa3\x89\x95\x26\xdb\x44\x3f\x99\xde\x95\xe9\xad\x30\xb6\x19\x2e\x4d\x76\x56\xfa\x8a\x50\x8d\x54\x14\x87\x40\xd0\xa6\xf0\x1d\xda\x95\xdc\xd6\x0b\x57\x40\xa3\x44\x6b\x40\x2a\x03\x9a\x5d\x5d\x61\x2c\xe4\x42\x3d\xaf\xfd\x7a\xa2\xe5\x94\x72\x68\x59\x71\xb8\xb7\xf6\x9f\xaf\x7c\x4d\x66\x9e\x76\x44\xff\xe2\xd0\xb3\x6b\xf3\xdb\xfb\xd4\x73\x08\xdd\x7a\xe3\xd8\xd3\x8e\xa4\xc5\xb9\x67\xa1\x98\xd8\x28\xcd\xb5\x3e\xcc\xad\xdb\xa9\xd0\xf9\x72\xf9\x89\x2d\x1f\xac\xa9\xd5\x69\x6e\x92\x99\x32\x48\xdf\x00\x06\x17\x2f\x80\x64\x86\xee\x3b\x18\xb7\x54\xdd\xeb\x9e\x7b\x45\x5d\x0f\x38\xf1\xe9\x7b\x0c\xbe\x21\x2d\x71\xce\xbc\x0f\x41\x4a\xca\x7c\xfe\x95\x2d\x70\x57\xde\x72\xfb\x40\xdd\xd4\xd2\x2d\x22\xa3\x2a\x03\xff\x31\x31\x1a\xe2\x3b\x8c\x87\xa1\x7a\x85\x60\xc6\xc3\x04\xca\xad\xb6\x24\xf6\x6f\xbe\x31\xaf\x22\xa8\x39\x33\x52\x48\x80\x74\xa9\x74\xc4\xe9\x5a\x9a\x88\x93\x78\xda\xac\x36\x46\x93\x3c\x3e\x4a\x21\x01\xe9\x2b\xc7\xe0\x55\x06\xd4\x40\xed\x1e\x35\x84\xa9\x83\x46\x4f\x1c\x11\xc2\x26\x2a\xcf\xa8\x5e\xb1\xe6\x60\x55\x54\xdc\x0f\xcb\xa1\x59\xed\x03\x7b\xc9\x8d\xba\x52\x69\x23\x29\xc6\x18\x2f\x8a\xd8\x26\xb7\xdf\x24\x54\x7f\xae\x53\xaf\xef\xa1\x0c\x34\x88\x78\xa5\x87\x16\xd8\xf2\xff\x4e\x70\xd7\xcd\x71\x9d\x03\x7f\x1e\x6d\xb7\x01\x64\x5e\x4d\xb3\x61\x9e\x11\x71\xe0\xff\x0e\x00\x00\xff\xff\x2a\x62\x9d\xe1\x81\x15\x00\x00"),
		},
		"/ContentDirectory.xml": &vfsgen۰CompressedFileInfo{
			name:             "ContentDirectory.xml",
			modTime:          time.Date(2019, 5, 26, 17, 14, 34, 746328001, time.UTC),
			uncompressedSize: 12994,

			compressedContent: []byte("\x1f\x8b\x08\x00\x00\x00\x00\x00\x00\xff\xec\x5a\x5b\x4f\xeb\x38\x10\x7e\x2e\xbf\x02\xf5\x9d\x0d\x48\xfb\x74\x64\x7a\x74\xb6\x2d\xa8\x12\xd0\x2a\xed\x41\xda\x27\x64\x92\x39\xad\x8f\x52\x3b\x3b\x9e\x40\xf9\xf7\xab\xdc\xda\xa4\x24\x94\x26\x4e\x36\x8b\xfa\x96\x9b\xbf\xef\x9b\xf1\x78\x26\xbe\xb0\xef\x9b\xb5\x77\xfe\x02\xa8\x85\x92\xd7\xfd\xab\x3f\x2e\xfb\xdf\x07\x67\x4c\x3b\xbe\x7b\xbe\x59\x7b\x52\x5f\xf7\x03\x94\xdf\xb4\xb3\x82\x35\xd7\x17\x81\x2f\xfd\x0b\x85\xcb\x6f\x1a\xf0\x45\x38\x70\x71\x75\x71\xd9\x1f\x9c\xf5\x98\xf6\xc1\x79\x8c\x51\x06\x67\xbd\x1e\x5b\xf3\xdf\x0a\x07\x57\xcc\x8a\x2f\xa2\x47\x42\x2a\x1c\x5c\x32\x2b\xbe\x38\xeb\x31\x2b\xdf\x8a\x71\x87\x84\x92\x77\x42\x53\xd4\x20\xbe\x0d\x2f\x7b\x4c\xf2\x35\x0c\x6e\x81\xe6\xc0\xd1\x59\x0d\xb9\xcf\x9f\x85\x27\x48\x80\x66\x56\xf4\x2e\xfa\x8a\xe3\x32\x58\x83\xa4\x14\x22\xf3\x28\xbe\x4d\x80\xb6\x28\xd9\xd6\xbd\x1e\x73\x05\x42\x4c\xaa\x02\x62\xd6\xee\x36\x79\x8f\xe0\x71\x02\x77\x4e\x9c\xe0\x91\xa3\xe0\xcf\x5e\x06\x2c\x23\xa9\xf0\xc3\x58\x90\x95\x53\xb4\xbb\xdd\x9a\x6d\xed\xec\x2e\x76\x81\x42\xaa\xed\x80\x18\xc3\x88\xf9\xef\xe4\x34\x6d\xfc\x78\x43\x20\xc3\x98\x31\xe1\x85\x2c\x98\x29\x77\x94\x08\x6c\xd2\x2f\x37\xc0\x29\x40\x08\xbf\xaf\xe2\x89\xe2\xe6\x55\x7d\x90\x43\x6b\x34\x1a\xde\x34\xc1\xfa\xa7\xef\x72\x82\xc9\xa8\x8a\xe1\x13\xd7\x44\x9f\xef\xc9\x68\xcc\xe4\xbf\x50\xbd\x6a\xa8\x62\xe7\xf4\xf9\x37\x38\x94\xf3\x51\xce\x5a\x21\x3f\x6b\xec\x8f\xa7\x1f\xf6\xed\xd3\xe2\xef\xd9\xf8\x69\x07\xfa\x69\x8b\x4b\xe4\xc5\x86\xdd\x78\x7c\x69\x54\x60\x16\xb6\xae\xc4\x1b\xe1\x11\xa0\x51\x79\x29\x64\x5d\x69\x73\xe2\x48\x42\x2e\x27\xd2\x85\x8d\x51\x85\x09\x62\x5d\x81\x36\xfc\x13\x80\x26\x70\x87\x2a\x90\xa5\x19\xa6\x92\xc2\x04\xb1\xb6\x0b\xc3\x2a\x86\x82\x00\x05\x37\xaa\x2f\x0f\x5c\xdf\x91\x3a\xf0\x4c\xa4\xe8\x8c\xc2\x14\xb3\xae\xb6\x87\x60\xfd\x0c\x68\x03\x05\x28\xc1\x44\x5a\x35\xdf\xcb\x0b\x45\xdc\xbb\xe7\xe4\xac\xc0\x44\xad\x37\x2f\xb0\xa0\x94\x19\x10\xd7\x42\x65\x8a\x7f\x82\xab\x54\xa6\xa1\x92\xc4\x85\x04\xec\x6c\x71\x4a\x7e\xf0\x1b\xc9\x0e\x7b\xd0\xa7\x22\x75\x2a\x52\xa7\x22\x75\x2a\x52\xa7\x22\xd5\x44\x91\x1a\x22\x70\x82\xb8\x30\x7c\xcd\x52\x35\xf6\x20\x7c\x56\x1a\x37\x95\xe4\x99\x1a\x7b\x87\xe6\xa0\xd5\xe2\xc6\x9c\xf3\x3a\x91\xb7\x8e\x8d\xe9\x11\x68\x42\xf5\x56\x3d\xa8\xbb\xb2\x32\x70\xac\xe1\x71\xbe\xf8\xff\xdb\x5d\x96\x6b\x02\x44\x90\xb4\xe0\xcb\x47\xee\x05\x60\x54\x65\x0a\x7a\xe4\x02\x5d\x59\x49\x85\xd7\x2e\xa9\x3c\x36\x8e\xee\xd5\xcb\xd7\x8d\xa2\x07\x78\x9d\xf1\x30\x8e\xba\xac\xb0\x33\x75\xe1\xd8\xd0\x99\xac\x7d\x85\x64\x83\x56\x01\x3a\x95\x96\x65\xe7\x51\xcb\x9f\xf6\xc4\x68\xef\x44\x78\x75\x3b\x26\x2c\x2c\x42\xf2\x90\xb7\x93\xfa\x16\xc8\xa5\xfe\xf5\xd1\xbf\x58\xb5\xb8\xc9\xe2\x36\x16\x39\xe3\xcd\x29\x72\x4e\x91\x53\x25\x72\xe6\xa4\xfc\x94\xa8\x4e\xfc\x1c\x76\x42\xb5\xa2\xdd\x86\x0f\x46\xe0\x01\x41\x1d\xeb\xd3\xb6\xff\x6d\x7c\x56\xd8\xf1\x4c\xfd\x3b\x43\xb5\x44\xd0\x95\xb6\xbd\xbb\xd4\xf5\x07\x24\x86\x00\x81\xe1\x55\x90\x7d\x6c\x53\x5a\xef\x40\x2e\x69\xd5\x8c\xd6\x14\xdb\x94\xd6\x68\x8d\xa9\x19\xa9\x09\x74\xc3\x8b\x38\x36\xfc\x02\x04\x59\x6d\xf4\x77\x7f\x1d\xa7\xfb\xd3\x8a\xae\xff\xae\xa7\x97\xc9\x2b\x96\x1c\x58\x8b\x50\x17\x29\x24\xd3\x59\x92\x73\x0d\xd2\x1d\xbf\x80\x24\x7d\xdd\x97\xaa\x9f\xad\xba\x1f\x1e\x3b\x73\x39\xf1\xc5\x9b\x0f\x03\x4d\x28\xe4\x92\x59\xdb\x07\x91\x26\xbd\x6f\xc9\xe7\x69\x3f\x38\xea\xd5\x28\xe9\xc1\x23\x56\x06\xd9\xdf\x40\xe7\xe8\x4b\xcf\xf4\x6c\x29\x02\xf1\xa7\x41\xc2\x6d\x36\x48\x39\x5b\x32\x74\x57\x2f\xdb\xe9\xd7\x92\x23\x62\x8d\xf1\x15\x0e\xee\x76\x79\xdf\xad\xe8\xb6\xc2\x5a\xba\x5f\xdc\x0a\x7b\xe1\x51\xaa\x8f\x99\x7b\x8c\x7b\x9e\x7a\x05\x77\xbb\xde\x96\x26\xff\xcc\xe3\xe4\x8c\xd6\x3d\x10\x0f\xdb\x32\x2b\xf7\xb2\xf4\xfb\x51\x54\x08\x86\x2b\xe1\xb9\x08\xb2\xa0\x55\xfe\xd1\x2e\x93\x1b\x71\xc6\xbb\xfd\xf0\x76\x02\xa0\x78\x9f\xb6\x15\xee\xfd\xed\x75\x63\x59\xb3\x94\x71\x7f\xbb\xbc\x79\xc6\x26\x8b\x43\x29\x69\xe1\xac\xa9\x3d\xda\x77\x33\x21\x13\x43\x7a\x38\xbd\x9f\xdd\x8d\x17\xe3\xd1\xe1\xd1\x3c\xb6\xed\xa9\x7d\xf8\xb3\xc9\xc3\xd3\xcc\x9e\xde\xda\xe3\xf9\xfc\xf0\xc7\xf3\xc5\x74\x36\x2b\x24\x6f\x34\x29\x94\x4e\xd8\x5a\x19\xa0\x65\x53\xb0\x76\xc8\x73\x7b\x2a\xed\x72\xe7\xd7\x5a\x32\x23\x07\xc5\x21\x3e\x66\x15\xfc\xbe\x33\x4b\x3b\xbe\x3b\xf8\x37\x00\x00\xff\xff\x6a\xc0\xaf\xb9\xc2\x32\x00\x00"),
		},
	}
	fs["/"].(*vfsgen۰DirInfo).entries = []os.FileInfo{
		fs["/ConnectionManager.xml"].(os.FileInfo),
		fs["/ContentDirectory.xml"].(os.FileInfo),
	}

	return fs
}()

type vfsgen۰FS map[string]interface{}

func (fs vfsgen۰FS) Open(path string) (http.File, error) {
	path = pathpkg.Clean("/" + path)
	f, ok := fs[path]
	if !ok {
		return nil, &os.PathError{Op: "open", Path: path, Err: os.ErrNotExist}
	}

	switch f := f.(type) {
	case *vfsgen۰CompressedFileInfo:
		gr, err := gzip.NewReader(bytes.NewReader(f.compressedContent))
		if err != nil {
			// This should never happen because we generate the gzip bytes such that they are always valid.
			panic("unexpected error reading own gzip compressed bytes: " + err.Error())
		}
		return &vfsgen۰CompressedFile{
			vfsgen۰CompressedFileInfo: f,
			gr:                        gr,
		}, nil
	case *vfsgen۰DirInfo:
		return &vfsgen۰Dir{
			vfsgen۰DirInfo: f,
		}, nil
	default:
		// This should never happen because we generate only the above types.
		panic(fmt.Sprintf("unexpected type %T", f))
	}
}

// vfsgen۰CompressedFileInfo is a static definition of a gzip compressed file.
type vfsgen۰CompressedFileInfo struct {
	name              string
	modTime           time.Time
	compressedContent []byte
	uncompressedSize  int64
}

func (f *vfsgen۰CompressedFileInfo) Readdir(count int) ([]os.FileInfo, error) {
	return nil, fmt.Errorf("cannot Readdir from file %s", f.name)
}
func (f *vfsgen۰CompressedFileInfo) Stat() (os.FileInfo, error) { return f, nil }

func (f *vfsgen۰CompressedFileInfo) GzipBytes() []byte {
	return f.compressedContent
}

func (f *vfsgen۰CompressedFileInfo) Name() string       { return f.name }
func (f *vfsgen۰CompressedFileInfo) Size() int64        { return f.uncompressedSize }
func (f *vfsgen۰CompressedFileInfo) Mode() os.FileMode  { return 0444 }
func (f *vfsgen۰CompressedFileInfo) ModTime() time.Time { return f.modTime }
func (f *vfsgen۰CompressedFileInfo) IsDir() bool        { return false }
func (f *vfsgen۰CompressedFileInfo) Sys() interface{}   { return nil }

// vfsgen۰CompressedFile is an opened compressedFile instance.
type vfsgen۰CompressedFile struct {
	*vfsgen۰CompressedFileInfo
	gr      *gzip.Reader
	grPos   int64 // Actual gr uncompressed position.
	seekPos int64 // Seek uncompressed position.
}

func (f *vfsgen۰CompressedFile) Read(p []byte) (n int, err error) {
	if f.grPos > f.seekPos {
		// Rewind to beginning.
		err = f.gr.Reset(bytes.NewReader(f.compressedContent))
		if err != nil {
			return 0, err
		}
		f.grPos = 0
	}
	if f.grPos < f.seekPos {
		// Fast-forward.
		_, err = io.CopyN(ioutil.Discard, f.gr, f.seekPos-f.grPos)
		if err != nil {
			return 0, err
		}
		f.grPos = f.seekPos
	}
	n, err = f.gr.Read(p)
	f.grPos += int64(n)
	f.seekPos = f.grPos
	return n, err
}
func (f *vfsgen۰CompressedFile) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		f.seekPos = 0 + offset
	case io.SeekCurrent:
		f.seekPos += offset
	case io.SeekEnd:
		f.seekPos = f.uncompressedSize + offset
	default:
		panic(fmt.Errorf("invalid whence value: %v", whence))
	}
	return f.seekPos, nil
}
func (f *vfsgen۰CompressedFile) Close() error {
	return f.gr.Close()
}

// vfsgen۰DirInfo is a static definition of a directory.
type vfsgen۰DirInfo struct {
	name    string
	modTime time.Time
	entries []os.FileInfo
}

func (d *vfsgen۰DirInfo) Read([]byte) (int, error) {
	return 0, fmt.Errorf("cannot Read from directory %s", d.name)
}
func (d *vfsgen۰DirInfo) Close() error               { return nil }
func (d *vfsgen۰DirInfo) Stat() (os.FileInfo, error) { return d, nil }

func (d *vfsgen۰DirInfo) Name() string       { return d.name }
func (d *vfsgen۰DirInfo) Size() int64        { return 0 }
func (d *vfsgen۰DirInfo) Mode() os.FileMode  { return 0755 | os.ModeDir }
func (d *vfsgen۰DirInfo) ModTime() time.Time { return d.modTime }
func (d *vfsgen۰DirInfo) IsDir() bool        { return true }
func (d *vfsgen۰DirInfo) Sys() interface{}   { return nil }

// vfsgen۰Dir is an opened dir instance.
type vfsgen۰Dir struct {
	*vfsgen۰DirInfo
	pos int // Position within entries for Seek and Readdir.
}

func (d *vfsgen۰Dir) Seek(offset int64, whence int) (int64, error) {
	if offset == 0 && whence == io.SeekStart {
		d.pos = 0
		return 0, nil
	}
	return 0, fmt.Errorf("unsupported Seek in directory %s", d.name)
}

func (d *vfsgen۰Dir) Readdir(count int) ([]os.FileInfo, error) {
	if d.pos >= len(d.entries) && count > 0 {
		return nil, io.EOF
	}
	if count <= 0 || count > len(d.entries)-d.pos {
		count = len(d.entries) - d.pos
	}
	e := d.entries[d.pos : d.pos+count]
	d.pos += count
	return e, nil
}
