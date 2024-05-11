package operations

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/crypt"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
)

// ListJSONItem in the struct which gets marshalled for each line
type ListJSONItem struct {
	Path          string
	Name          string
	EncryptedPath string `json:",omitempty"`
	Encrypted     string `json:",omitempty"`
	Size          int64
	MimeType      string    `json:",omitempty"`
	ModTime       Timestamp //`json:",omitempty"`
	IsDir         bool
	Hashes        map[string]string `json:",omitempty"`
	ID            string            `json:",omitempty"`
	OrigID        string            `json:",omitempty"`
	Tier          string            `json:",omitempty"`
	IsBucket      bool              `json:",omitempty"`
	Metadata      fs.Metadata       `json:",omitempty"`
}

// Timestamp a time in the provided format
type Timestamp struct {
	When   time.Time
	Format string
}

// MarshalJSON turns a Timestamp into JSON
func (t Timestamp) MarshalJSON() (out []byte, err error) {
	if t.When.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + t.When.Format(t.Format) + `"`), nil
}

// Returns a time format for the given precision
func formatForPrecision(precision time.Duration) string {
	switch {
	case precision <= time.Nanosecond:
		return "2006-01-02T15:04:05.000000000Z07:00"
	case precision <= 10*time.Nanosecond:
		return "2006-01-02T15:04:05.00000000Z07:00"
	case precision <= 100*time.Nanosecond:
		return "2006-01-02T15:04:05.0000000Z07:00"
	case precision <= time.Microsecond:
		return "2006-01-02T15:04:05.000000Z07:00"
	case precision <= 10*time.Microsecond:
		return "2006-01-02T15:04:05.00000Z07:00"
	case precision <= 100*time.Microsecond:
		return "2006-01-02T15:04:05.0000Z07:00"
	case precision <= time.Millisecond:
		return "2006-01-02T15:04:05.000Z07:00"
	case precision <= 10*time.Millisecond:
		return "2006-01-02T15:04:05.00Z07:00"
	case precision <= 100*time.Millisecond:
		return "2006-01-02T15:04:05.0Z07:00"
	}
	return time.RFC3339
}

// ListJSONOpt describes the options for ListJSON
type ListJSONOpt struct {
	Recurse       bool     `json:"recurse"`
	NoModTime     bool     `json:"noModTime"`
	NoMimeType    bool     `json:"noMimeType"`
	ShowEncrypted bool     `json:"showEncrypted"`
	ShowOrigIDs   bool     `json:"showOrigIDs"`
	ShowHash      bool     `json:"showHash"`
	DirsOnly      bool     `json:"dirsOnly"`
	FilesOnly     bool     `json:"filesOnly"`
	Metadata      bool     `json:"metadata"`
	HashTypes     []string `json:"hashTypes"` // hash types to show if ShowHash is set, e.g. "MD5", "SHA-1"
}

// state for ListJson
type listJSON struct {
	fsrc       fs.Fs
	remote     string
	format     string
	opt        *ListJSONOpt
	cipher     *crypt.Cipher
	hashTypes  []hash.Type
	dirs       bool
	files      bool
	canGetTier bool
	isBucket   bool
	showHash   bool
}

func newListJSON(ctx context.Context, fsrc fs.Fs, remote string, opt *ListJSONOpt) (*listJSON, error) {
	lj := &listJSON{
		fsrc:   fsrc,
		remote: remote,
		opt:    opt,
		dirs:   true,
		files:  true,
	}
	//                       Dirs    Files
	// !FilesOnly,!DirsOnly  true    true
	// !FilesOnly,DirsOnly   true    false
	// FilesOnly,!DirsOnly   false   true
	// FilesOnly,DirsOnly    true    true
	if !opt.FilesOnly && opt.DirsOnly {
		lj.files = false
	} else if opt.FilesOnly && !opt.DirsOnly {
		lj.dirs = false
	}
	if opt.ShowEncrypted {
		fsInfo, _, _, config, err := fs.ConfigFs(fs.ConfigStringFull(fsrc))
		if err != nil {
			return nil, fmt.Errorf("ListJSON failed to load config for crypt remote: %w", err)
		}
		if fsInfo.Name != "crypt" {
			return nil, errors.New("the remote needs to be of type \"crypt\"")
		}
		lj.cipher, err = crypt.NewCipher(config)
		if err != nil {
			return nil, fmt.Errorf("ListJSON failed to make new crypt remote: %w", err)
		}
	}
	features := fsrc.Features()
	lj.canGetTier = features.GetTier
	lj.format = formatForPrecision(fsrc.Precision())
	lj.isBucket = features.BucketBased && remote == "" && fsrc.Root() == "" // if bucket-based remote listing the root mark directories as buckets
	lj.showHash = opt.ShowHash
	lj.hashTypes = fsrc.Hashes().Array()
	if len(opt.HashTypes) != 0 {
		lj.showHash = true
		lj.hashTypes = []hash.Type{}
		for _, hashType := range opt.HashTypes {
			var ht hash.Type
			err := ht.Set(hashType)
			if err != nil {
				return nil, err
			}
			lj.hashTypes = append(lj.hashTypes, ht)
		}
	}
	return lj, nil
}

// Convert a single entry to JSON
//
// It may return nil if there is no entry to return
func (lj *listJSON) entry(ctx context.Context, entry fs.DirEntry) (*ListJSONItem, error) {
	switch entry.(type) {
	case fs.Directory:
		if lj.opt.FilesOnly {
			return nil, nil
		}
	case fs.Object:
		if lj.opt.DirsOnly {
			return nil, nil
		}
	default:
		fs.Errorf(nil, "Unknown type %T in listing", entry)
	}

	item := &ListJSONItem{
		Path: entry.Remote(),
		Name: path.Base(entry.Remote()),
		Size: entry.Size(),
	}
	if entry.Remote() == "" {
		item.Name = ""
	}
	if !lj.opt.NoModTime {
		item.ModTime = Timestamp{When: entry.ModTime(ctx), Format: lj.format}
	}
	if !lj.opt.NoMimeType {
		item.MimeType = fs.MimeTypeDirEntry(ctx, entry)
	}
	if lj.cipher != nil {
		switch entry.(type) {
		case fs.Directory:
			item.EncryptedPath = lj.cipher.EncryptDirName(entry.Remote())
		case fs.Object:
			item.EncryptedPath = lj.cipher.EncryptFileName(entry.Remote())
		default:
			fs.Errorf(nil, "Unknown type %T in listing", entry)
		}
		item.Encrypted = path.Base(item.EncryptedPath)
	}
	if lj.opt.Metadata {
		metadata, err := fs.GetMetadata(ctx, entry)
		if err != nil {
			fs.Errorf(entry, "Failed to read metadata: %v", err)
		} else if metadata != nil {
			item.Metadata = metadata
		}
	}
	if do, ok := entry.(fs.IDer); ok {
		item.ID = do.ID()
	}
	if o, ok := entry.(fs.Object); lj.opt.ShowOrigIDs && ok {
		if do, ok := fs.UnWrapObject(o).(fs.IDer); ok {
			item.OrigID = do.ID()
		}
	}
	switch x := entry.(type) {
	case fs.Directory:
		item.IsDir = true
		item.IsBucket = lj.isBucket
	case fs.Object:
		item.IsDir = false
		if lj.showHash {
			item.Hashes = make(map[string]string)
			for _, hashType := range lj.hashTypes {
				hash, err := x.Hash(ctx, hashType)
				if err != nil {
					fs.Errorf(x, "Failed to read hash: %v", err)
				} else if hash != "" {
					item.Hashes[hashType.String()] = hash
				}
			}
		}
		if lj.canGetTier {
			if do, ok := x.(fs.GetTierer); ok {
				item.Tier = do.GetTier()
			}
		}
	default:
		fs.Errorf(nil, "Unknown type %T in listing in ListJSON", entry)
	}
	return item, nil
}

// ListJSON lists fsrc using the options in opt calling callback for each item
func ListJSON(ctx context.Context, fsrc fs.Fs, remote string, opt *ListJSONOpt, callback func(*ListJSONItem) error) error {
	lj, err := newListJSON(ctx, fsrc, remote, opt)
	if err != nil {
		return err
	}
	err = walk.ListR(ctx, fsrc, remote, false, ConfigMaxDepth(ctx, lj.opt.Recurse), walk.ListAll, func(entries fs.DirEntries) (err error) {
		for _, entry := range entries {
			item, err := lj.entry(ctx, entry)
			if err != nil {
				return fmt.Errorf("creating entry failed in ListJSON: %w", err)
			}
			if item != nil {
				err = callback(item)
				if err != nil {
					return fmt.Errorf("callback failed in ListJSON: %w", err)
				}
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error in ListJSON: %w", err)
	}
	return nil
}

// StatJSON returns a single JSON stat entry for the fsrc, remote path
//
// The item returned may be nil if it is not found or excluded with DirsOnly/FilesOnly
func StatJSON(ctx context.Context, fsrc fs.Fs, remote string, opt *ListJSONOpt) (item *ListJSONItem, err error) {
	// FIXME this could me more efficient we had a new primitive
	// NewDirEntry() which returned an Object or a Directory
	lj, err := newListJSON(ctx, fsrc, remote, opt)
	if err != nil {
		return nil, err
	}

	// Root is always a directory. When we have a NewDirEntry
	// primitive we need to call it, but for now this will do.
	if remote == "" {
		if !lj.dirs {
			return nil, nil
		}
		// Check the root directory exists
		_, err := fsrc.List(ctx, "")
		if err != nil {
			return nil, err
		}
		return lj.entry(ctx, fs.NewDir("", time.Now()))
	}

	// Could be a file or a directory here
	if lj.files && !strings.HasSuffix(remote, "/") {
		// NewObject can return the sentinel errors ErrorObjectNotFound or ErrorIsDir
		// ErrorObjectNotFound can mean the source is a directory or not found
		obj, err := fsrc.NewObject(ctx, remote)
		if err == fs.ErrorObjectNotFound {
			if !lj.dirs {
				return nil, nil
			}
		} else if err == fs.ErrorIsDir {
			if !lj.dirs {
				return nil, nil
			}
			// This could return a made up ListJSONItem here
			// but that wouldn't have the IDs etc in
		} else if err != nil {
			if !lj.dirs {
				return nil, err
			}
		} else {
			return lj.entry(ctx, obj)
		}
	}
	// Must be a directory here
	//
	// Remove trailing / as rclone listings won't have them
	remote = strings.TrimRight(remote, "/")
	parent := path.Dir(remote)
	if parent == "." || parent == "/" {
		parent = ""
	}
	entries, err := fsrc.List(ctx, parent)
	if err == fs.ErrorDirNotFound {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	equal := func(a, b string) bool { return a == b }
	if fsrc.Features().CaseInsensitive {
		equal = strings.EqualFold
	}
	var foundEntry fs.DirEntry
	for _, entry := range entries {
		if equal(entry.Remote(), remote) {
			foundEntry = entry
			break
		}
	}
	if foundEntry == nil {
		return nil, nil
	}
	return lj.entry(ctx, foundEntry)
}
