package operations

import (
	"context"
	"path"
	"time"

	"github.com/ncw/rclone/backend/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/walk"
	"github.com/pkg/errors"
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
	Recurse       bool `json:"recurse"`
	NoModTime     bool `json:"noModTime"`
	ShowEncrypted bool `json:"showEncrypted"`
	ShowOrigIDs   bool `json:"showOrigIDs"`
	ShowHash      bool `json:"showHash"`
	DirsOnly      bool `json:"dirsOnly"`
	FilesOnly     bool `json:"filesOnly"`
}

// ListJSON lists fsrc using the options in opt calling callback for each item
func ListJSON(ctx context.Context, fsrc fs.Fs, remote string, opt *ListJSONOpt, callback func(*ListJSONItem) error) error {
	var cipher crypt.Cipher
	if opt.ShowEncrypted {
		fsInfo, _, _, config, err := fs.ConfigFs(fsrc.Name() + ":" + fsrc.Root())
		if err != nil {
			return errors.Wrap(err, "ListJSON failed to load config for crypt remote")
		}
		if fsInfo.Name != "crypt" {
			return errors.New("The remote needs to be of type \"crypt\"")
		}
		cipher, err = crypt.NewCipher(config)
		if err != nil {
			return errors.Wrap(err, "ListJSON failed to make new crypt remote")
		}
	}
	features := fsrc.Features()
	canGetTier := features.GetTier
	format := formatForPrecision(fsrc.Precision())
	isBucket := features.BucketBased && remote == "" && fsrc.Root() == "" // if bucket based remote listing the root mark directories as buckets
	err := walk.ListR(ctx, fsrc, remote, false, ConfigMaxDepth(opt.Recurse), walk.ListAll, func(entries fs.DirEntries) (err error) {
		for _, entry := range entries {
			switch entry.(type) {
			case fs.Directory:
				if opt.FilesOnly {
					continue
				}
			case fs.Object:
				if opt.DirsOnly {
					continue
				}
			default:
				fs.Errorf(nil, "Unknown type %T in listing", entry)
			}

			item := ListJSONItem{
				Path:     entry.Remote(),
				Name:     path.Base(entry.Remote()),
				Size:     entry.Size(),
				MimeType: fs.MimeTypeDirEntry(ctx, entry),
			}
			if !opt.NoModTime {
				item.ModTime = Timestamp{When: entry.ModTime(ctx), Format: format}
			}
			if cipher != nil {
				switch entry.(type) {
				case fs.Directory:
					item.EncryptedPath = cipher.EncryptDirName(entry.Remote())
				case fs.Object:
					item.EncryptedPath = cipher.EncryptFileName(entry.Remote())
				default:
					fs.Errorf(nil, "Unknown type %T in listing", entry)
				}
				item.Encrypted = path.Base(item.EncryptedPath)
			}
			if do, ok := entry.(fs.IDer); ok {
				item.ID = do.ID()
			}
			if opt.ShowOrigIDs {
				cur := entry
				for {
					u, ok := cur.(fs.ObjectUnWrapper)
					if !ok {
						break // not a wrapped object, use current id
					}
					next := u.UnWrap()
					if next == nil {
						break // no base object found, use current id
					}
					cur = next
				}
				if do, ok := cur.(fs.IDer); ok {
					item.OrigID = do.ID()
				}
			}
			switch x := entry.(type) {
			case fs.Directory:
				item.IsDir = true
				item.IsBucket = isBucket
			case fs.Object:
				item.IsDir = false
				if opt.ShowHash {
					item.Hashes = make(map[string]string)
					for _, hashType := range x.Fs().Hashes().Array() {
						hash, err := x.Hash(ctx, hashType)
						if err != nil {
							fs.Errorf(x, "Failed to read hash: %v", err)
						} else if hash != "" {
							item.Hashes[hashType.String()] = hash
						}
					}
				}
				if canGetTier {
					if do, ok := x.(fs.GetTierer); ok {
						item.Tier = do.GetTier()
					}
				}
			default:
				fs.Errorf(nil, "Unknown type %T in listing in ListJSON", entry)
			}
			err = callback(&item)
			if err != nil {
				return errors.Wrap(err, "callback failed in ListJSON")
			}

		}
		return nil
	})
	if err != nil {
		return errors.Wrap(err, "error in ListJSON")
	}
	return nil
}
