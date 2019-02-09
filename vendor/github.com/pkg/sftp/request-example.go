package sftp

// This serves as an example of how to implement the request server handler as
// well as a dummy backend for testing. It implements an in-memory backend that
// works as a very simple filesystem with simple flat key-value lookup system.

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"
)

// InMemHandler returns a Hanlders object with the test handlers.
func InMemHandler() Handlers {
	root := &root{
		files: make(map[string]*memFile),
	}
	root.memFile = newMemFile("/", true)
	return Handlers{root, root, root, root}
}

// Example Handlers
func (fs *root) Fileread(r *Request) (io.ReaderAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}
	_ = r.WithContext(r.Context()) // initialize context for deadlock testing
	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()
	file, err := fs.fetch(r.Filepath)
	if err != nil {
		return nil, err
	}
	if file.symlink != "" {
		file, err = fs.fetch(file.symlink)
		if err != nil {
			return nil, err
		}
	}
	return file.ReaderAt()
}

func (fs *root) Filewrite(r *Request) (io.WriterAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}
	_ = r.WithContext(r.Context()) // initialize context for deadlock testing
	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()
	file, err := fs.fetch(r.Filepath)
	if err == os.ErrNotExist {
		dir, err := fs.fetch(filepath.Dir(r.Filepath))
		if err != nil {
			return nil, err
		}
		if !dir.isdir {
			return nil, os.ErrInvalid
		}
		file = newMemFile(r.Filepath, false)
		fs.files[r.Filepath] = file
	}
	return file.WriterAt()
}

func (fs *root) Filecmd(r *Request) error {
	if fs.mockErr != nil {
		return fs.mockErr
	}
	_ = r.WithContext(r.Context()) // initialize context for deadlock testing
	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()
	switch r.Method {
	case "Setstat":
		return nil
	case "Rename":
		file, err := fs.fetch(r.Filepath)
		if err != nil {
			return err
		}
		if _, ok := fs.files[r.Target]; ok {
			return &os.LinkError{Op: "rename", Old: r.Filepath, New: r.Target,
				Err: fmt.Errorf("dest file exists")}
		}
		file.name = r.Target
		fs.files[r.Target] = file
		delete(fs.files, r.Filepath)
	case "Rmdir", "Remove":
		_, err := fs.fetch(filepath.Dir(r.Filepath))
		if err != nil {
			return err
		}
		delete(fs.files, r.Filepath)
	case "Mkdir":
		_, err := fs.fetch(filepath.Dir(r.Filepath))
		if err != nil {
			return err
		}
		fs.files[r.Filepath] = newMemFile(r.Filepath, true)
	case "Symlink":
		_, err := fs.fetch(r.Filepath)
		if err != nil {
			return err
		}
		link := newMemFile(r.Target, false)
		link.symlink = r.Filepath
		fs.files[r.Target] = link
	}
	return nil
}

type listerat []os.FileInfo

// Modeled after strings.Reader's ReadAt() implementation
func (f listerat) ListAt(ls []os.FileInfo, offset int64) (int, error) {
	var n int
	if offset >= int64(len(f)) {
		return 0, io.EOF
	}
	n = copy(ls, f[offset:])
	if n < len(ls) {
		return n, io.EOF
	}
	return n, nil
}

func (fs *root) Filelist(r *Request) (ListerAt, error) {
	if fs.mockErr != nil {
		return nil, fs.mockErr
	}
	_ = r.WithContext(r.Context()) // initialize context for deadlock testing
	fs.filesLock.Lock()
	defer fs.filesLock.Unlock()

	file, err := fs.fetch(r.Filepath)
	if err != nil {
		return nil, err
	}

	switch r.Method {
	case "List":
		if !file.IsDir() {
			return nil, syscall.ENOTDIR
		}
		ordered_names := []string{}
		for fn, _ := range fs.files {
			if filepath.Dir(fn) == r.Filepath {
				ordered_names = append(ordered_names, fn)
			}
		}
		sort.Strings(ordered_names)
		list := make([]os.FileInfo, len(ordered_names))
		for i, fn := range ordered_names {
			list[i] = fs.files[fn]
		}
		return listerat(list), nil
	case "Stat":
		return listerat([]os.FileInfo{file}), nil
	case "Readlink":
		if file.symlink != "" {
			file, err = fs.fetch(file.symlink)
			if err != nil {
				return nil, err
			}
		}
		return listerat([]os.FileInfo{file}), nil
	}
	return nil, nil
}

// In memory file-system-y thing that the Hanlders live on
type root struct {
	*memFile
	files     map[string]*memFile
	filesLock sync.Mutex
	mockErr   error
}

// Set a mocked error that the next handler call will return.
// Set to nil to reset for no error.
func (fs *root) returnErr(err error) {
	fs.mockErr = err
}

func (fs *root) fetch(path string) (*memFile, error) {
	if path == "/" {
		return fs.memFile, nil
	}
	if file, ok := fs.files[path]; ok {
		return file, nil
	}
	return nil, os.ErrNotExist
}

// Implements os.FileInfo, Reader and Writer interfaces.
// These are the 3 interfaces necessary for the Handlers.
type memFile struct {
	name        string
	modtime     time.Time
	symlink     string
	isdir       bool
	content     []byte
	contentLock sync.RWMutex
}

// factory to make sure modtime is set
func newMemFile(name string, isdir bool) *memFile {
	return &memFile{
		name:    name,
		modtime: time.Now(),
		isdir:   isdir,
	}
}

// Have memFile fulfill os.FileInfo interface
func (f *memFile) Name() string { return filepath.Base(f.name) }
func (f *memFile) Size() int64  { return int64(len(f.content)) }
func (f *memFile) Mode() os.FileMode {
	ret := os.FileMode(0644)
	if f.isdir {
		ret = os.FileMode(0755) | os.ModeDir
	}
	if f.symlink != "" {
		ret = os.FileMode(0777) | os.ModeSymlink
	}
	return ret
}
func (f *memFile) ModTime() time.Time { return f.modtime }
func (f *memFile) IsDir() bool        { return f.isdir }
func (f *memFile) Sys() interface{} {
	return fakeFileInfoSys()
}

// Read/Write
func (f *memFile) ReaderAt() (io.ReaderAt, error) {
	if f.isdir {
		return nil, os.ErrInvalid
	}
	return bytes.NewReader(f.content), nil
}

func (f *memFile) WriterAt() (io.WriterAt, error) {
	if f.isdir {
		return nil, os.ErrInvalid
	}
	return f, nil
}
func (f *memFile) WriteAt(p []byte, off int64) (int, error) {
	// fmt.Println(string(p), off)
	// mimic write delays, should be optional
	time.Sleep(time.Microsecond * time.Duration(len(p)))
	f.contentLock.Lock()
	defer f.contentLock.Unlock()
	plen := len(p) + int(off)
	if plen >= len(f.content) {
		nc := make([]byte, plen)
		copy(nc, f.content)
		f.content = nc
	}
	copy(f.content[off:], p)
	return len(p), nil
}
