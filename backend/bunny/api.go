package bunny

import (
	"context"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/rest"
)

// DirList holds a cached directory listing
type DirList struct {
	dir   string
	items []DirItem
}

// DirItem represents a single entry in a Bunny storage directory listing
type DirItem struct {
	Guid            string // The unique identifier for this file
	StorageZoneName string // Name of the storage zone
	Path            string // Path to this file
	ObjectName      string // Filename
	Length          int64  // Size of the file
	LastChanged     string // Timestamp file was uploaded
	ServerId        int
	ArrayNumber     int
	IsDirectory     bool   // Entry is for a directory
	UserId          string // UUID of user who created file
	ContentType     string // File MIME Type
	DateCreated     string // Date file was first uploaded
	StorageZoneId   int    // Numeric ID of the storage zone
	Checksum        string // SHA256 checksum of file contents (uppercase hex)
	ReplicatedZones string // Zone names
}

// ModTime parses the LastChanged field into a time.Time
func (i *DirItem) ModTime() time.Time {
	// Bunny returns times like "2024-01-15T10:30:00.000" or "2024-01-15T10:30:00.000Z"
	s := strings.TrimSuffix(i.LastChanged, "Z")
	t, err := time.Parse("2006-01-02T15:04:05.999", s)
	if err != nil {
		t, err = time.Parse("2006-01-02T15:04:05", s)
		if err != nil {
			return time.Now()
		}
	}
	return t.UTC()
}

// list retrieves a directory listing from Bunny
func (f *Fs) list(ctx context.Context, dir string) (list *DirList, err error) {
	reqPath := f.getFullFilePath(dir, false)
	var response []DirItem
	opts := rest.Opts{
		Method:       "GET",
		Path:         reqPath + "/",
		ExtraHeaders: map[string]string{"Accept": "application/json"},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.CallJSON(ctx, &opts, nil, &response)
		if resp != nil && resp.StatusCode == 404 {
			return false, fs.ErrorDirNotFound
		}
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	list = &DirList{
		dir:   dir,
		items: response,
	}
	return list, nil
}

