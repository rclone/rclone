package nfs

import (
	"time"
)

// FileTime is the NFS wire time format
// This is equivalent to go-nfs-client/nfs.NFS3Time
type FileTime struct {
	Seconds  uint32
	Nseconds uint32
}

// ToNFSTime generates the nfs 64bit time format from a golang time.
func ToNFSTime(t time.Time) FileTime {
	return FileTime{
		Seconds:  uint32(t.Unix()),
		Nseconds: uint32(t.UnixNano() % int64(time.Second)),
	}
}

// Native generates a golang time from an nfs time spec
func (t FileTime) Native() *time.Time {
	ts := time.Unix(int64(t.Seconds), int64(t.Nseconds))
	return &ts
}

// EqualTimespec returns if this time is equal to a local time spec
func (t FileTime) EqualTimespec(sec int64, nsec int64) bool {
	// TODO: bounds check on sec/nsec overflow
	return t.Nseconds == uint32(nsec) && t.Seconds == uint32(sec)
}
