package kvfs

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/kv"
)

// An Fs is a representation of a remote KVFS Fs
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	db       *kv.DB
}

// An File is a representation of an actual file in the KVFS store
type File struct {
	Filename string
	Type     string
	Content  string
	Size     int64
	ModTime  int64
	SHA1     string
}

// An Object on the remote KVFS Fs
type Object struct {
	fs      *Fs       // what this object is part of
	info    File      // info about the object
	remote  string    // The remote path
	size    int64     // size of the object
	modTime time.Time // modification time of the object
	sha1    string    // SHA-1 of the object content
}

// Options represent the configuration of the KVFS backend
type Options struct {
	ConfigDir string
}

func (f *Fs) getDb() (*kv.DB, error) {
	var err error
	if f.db == nil {
		f.db, err = kv.Start(context.Background(), "kvfs", filepath.Join(f.opt.ConfigDir, "db"), f)
		if err != nil {
			return nil, fmt.Errorf("failed to insert file: %w", err)
		}
		if err != nil {
			return nil, err
		}
	}
	return f.db, nil
}

func (f *Fs) findFile(fullPath string) (*File, error) {
	fs.Debugf(nil, "[findFile] fullPath: %q", fullPath)
	var file File
	err := f.db.Do(false, &opGet{
		key:   fullPath,
		value: &file,
	})
	if err == kv.ErrEmpty {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	return &file, nil
}

func (f *Fs) fileExists(fullPath string) bool {
	file, err := f.findFile(fullPath)
	if err != nil {
		return false
	}
	return file != nil
}

func (f *Fs) getFiles(fullPath string) (*[]File, error) {
	dirExists := fullPath == "/"

	var files []File
	err := f.db.Do(false, &opList{
		prefix: fullPath,
		fn: func(key string, value []byte) error {
			var file File
			if key == "NewFs" {
				return nil
			}
			if err := json.Unmarshal(value, &file); err != nil {
				return err
			}
			if file.Filename == fullPath {
				dirExists = true
				return nil
			}
			dir := path.Dir(file.Filename)
			if dir == fullPath {
				files = append(files, file)
			}
			return nil
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	if !dirExists {
		return nil, fs.ErrorDirNotFound
	}

	return &files, nil
}

func (f *Fs) hasFiles(fullPath string) (bool, error) {
	files, err := f.getFiles(fullPath)
	if err != nil {
		return false, err
	}
	return len(*files) > 0, nil
}

func (f *Fs) mkDir(fullPath string) error {
	parts := strings.Split(fullPath, "/")

	for i, part := range parts {
		if part == "" {
			continue
		}
		dir := strings.Join(parts[:i+1], "/")
		if f.fileExists(dir) {
			continue
		}
		fs.Debugf(nil, "[mkdirTree] creating directory: %q part: %q", dir, part)

		file := &File{
			Filename: dir,
			Type:     "dir",
			ModTime:  time.Now().UnixMilli(),
		}

		data, err := json.Marshal(file)
		if err != nil {
			return fmt.Errorf("failed to marshal directory: %w", err)
		}

		err = f.db.Do(true, &opPut{
			key:   dir,
			value: data,
		})
		if err != nil {
			return fmt.Errorf("failed to insert directory: %w", err)
		}
	}
	return nil
}

func (f *Fs) rmDir(fullPath string) error {
	fs.Debugf(nil, "[rmdir] fullPath: %q", fullPath)

	// Check if directory is empty
	result, err := f.hasFiles(fullPath)
	if err != nil {
		return err
	}
	if result {
		return fs.ErrorDirectoryNotEmpty
	}

	// Check if directory exists
	file, err := f.findFile(fullPath)
	if err != nil {
		return err
	}
	if file == nil {
		return fs.ErrorDirNotFound
	}

	err = f.db.Do(true, &opDelete{
		key: fullPath,
	})
	if err != nil {
		return fmt.Errorf("failed to delete directory: %w", err)
	}
	return nil
}

func (f *Fs) putFile(in io.Reader, fullPath string, modTime time.Time) (*File, error) {
	fs.Debugf(nil, "[putFile] fullPath: %q", fullPath)

	content, err := func() (string, error) {
		data, err := io.ReadAll(in)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}()
	if err != nil {
		return nil, err
	}

	file := &File{
		Filename: fullPath,
		Type:     "file",
		ModTime:  modTime.UnixMilli(),
		Content:  content,
	}

	file.calculateMetadata()

	data, err := json.Marshal(file)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal file: %w", err)
	}

	err = f.db.Do(true, &opPut{
		key:   fullPath,
		value: data,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to insert file: %w", err)
	}

	return file, nil
}

func (f *Fs) remove(fullPath string) error {
	fs.Debugf(nil, "[remove] fullPath: %q", fullPath)
	err := f.db.Do(true, &opDelete{
		key: fullPath,
	})
	if err != nil {
		return fmt.Errorf("failed to delete file: %w", err)
	}
	return nil
}

// Calculate size and SHA1 hash for a file
func (f *File) calculateMetadata() {
	// Calculate size from content in bytes
	f.Size = int64(len(f.Content))
	// f.Size = int64(len([]byte(f.Content)))

	// Create a new SHA-1 hash object
	hasher := sha1.New()
	// Write the input string to the hasher
	hasher.Write([]byte(f.Content))
	f.SHA1 = hex.EncodeToString(hasher.Sum(nil))
}

// fullPath constructs a full, absolute path from an Fs root relative path,
func (f *Fs) fullPath(part string) string {
	return path.Join(f.root, part)
}

// KVFS store operations
type opGet struct {
	key   string
	value interface{}
}

func (op *opGet) Do(ctx context.Context, b kv.Bucket) error {
	data := b.Get([]byte(op.key))
	if data == nil {
		return kv.ErrEmpty
	}
	return json.Unmarshal(data, op.value)
}

type opPut struct {
	key   string
	value []byte
}

func (op *opPut) Do(ctx context.Context, b kv.Bucket) error {
	return b.Put([]byte(op.key), op.value)
}

type opDelete struct {
	key string
}

func (op *opDelete) Do(ctx context.Context, b kv.Bucket) error {
	return b.Delete([]byte(op.key))
}

type opList struct {
	prefix string
	fn     func(key string, value []byte) error
}

func (op *opList) Do(ctx context.Context, b kv.Bucket) error {
	c := b.Cursor()
	for k, v := c.Seek([]byte(op.prefix)); k != nil && strings.HasPrefix(string(k), op.prefix); k, v = c.Next() {
		if err := op.fn(string(k), v); err != nil {
			return err
		}
	}
	return nil
}
