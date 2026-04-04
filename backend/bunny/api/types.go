// Package api has type definitions for bunny.net API
package api

import (
	"strings"
	"time"
)

// DirList holds a cached directory listing
type DirList struct {
	Dir   string
	Items []DirItem
}

// DirItem represents a single entry in a Bunny storage directory listing
type DirItem struct {
	GUID            string // The unique identifier for this file
	StorageZoneName string // Name of the storage zone
	Path            string // Path to this file
	ObjectName      string // Filename
	Length          int64  // Size of the file
	LastChanged     string // Timestamp file was uploaded
	ServerID        int
	ArrayNumber     int
	IsDirectory     bool   // Entry is for a directory
	UserID          string // UUID of user who created file
	ContentType     string // File MIME Type
	DateCreated     string // Date file was first uploaded
	StorageZoneID   int    // Numeric ID of the storage zone
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
