package operations

import (
	"path"
	"time"

	"github.com/ncw/rclone/backend/crypt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/walk"
	"github.com/pkg/errors"
)

// ListJSONItem in the struct which gets marshalled for each line
type ListJSONItem struct {
	Path      string
	Name      string
	Encrypted string `json:",omitempty"`
	Size      int64
	MimeType  string    `json:",omitempty"`
	ModTime   Timestamp //`json:",omitempty"`
	IsDir     bool
	Hashes    map[string]string `json:",omitempty"`
	ID        string            `json:",omitempty"`
	OrigID    string            `json:",omitempty"`
}

// Timestamp a time in RFC3339 format with Nanosecond precision secongs
type Timestamp time.Time

// MarshalJSON turns a Timestamp into JSON
func (t Timestamp) MarshalJSON() (out []byte, err error) {
	tt := time.Time(t)
	if tt.IsZero() {
		return []byte(`""`), nil
	}
	return []byte(`"` + tt.Format(time.RFC3339Nano) + `"`), nil
}

// ListJSONOpt describes the options for ListJSON
type ListJSONOpt struct {
	Recurse       bool `json:"recurse"`
	NoModTime     bool `json:"noModTime"`
	ShowEncrypted bool `json:"showEncrypted"`
	ShowOrigIDs   bool `json:"showOrigIDs"`
	ShowHash      bool `json:"showHash"`
}

// ListJSON lists fsrc using the options in opt calling callback for each item
func ListJSON(fsrc fs.Fs, remote string, opt *ListJSONOpt, callback func(*ListJSONItem) error) error {
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
	err := walk.Walk(fsrc, remote, false, ConfigMaxDepth(opt.Recurse), func(dirPath string, entries fs.DirEntries, err error) error {
		if err != nil {
			fs.CountError(err)
			fs.Errorf(dirPath, "error listing: %v", err)
			return nil
		}
		for _, entry := range entries {
			item := ListJSONItem{
				Path:     entry.Remote(),
				Name:     path.Base(entry.Remote()),
				Size:     entry.Size(),
				MimeType: fs.MimeTypeDirEntry(entry),
			}
			if !opt.NoModTime {
				item.ModTime = Timestamp(entry.ModTime())
			}
			if cipher != nil {
				switch entry.(type) {
				case fs.Directory:
					item.Encrypted = cipher.EncryptDirName(path.Base(entry.Remote()))
				case fs.Object:
					item.Encrypted = cipher.EncryptFileName(path.Base(entry.Remote()))
				default:
					fs.Errorf(nil, "Unknown type %T in listing", entry)
				}
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
			case fs.Object:
				item.IsDir = false
				if opt.ShowHash {
					item.Hashes = make(map[string]string)
					for _, hashType := range x.Fs().Hashes().Array() {
						hash, err := x.Hash(hashType)
						if err != nil {
							fs.Errorf(x, "Failed to read hash: %v", err)
						} else if hash != "" {
							item.Hashes[hashType.String()] = hash
						}
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
