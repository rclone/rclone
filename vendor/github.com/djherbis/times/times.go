// Package times provides a platform-independent way to get atime, mtime, ctime and btime for files.
package times

import (
	"os"
	"time"
)

// Get returns the Timespec for the given FileInfo
func Get(fi os.FileInfo) Timespec {
	return getTimespec(fi)
}

// Stat returns the Timespec for the given filename.
func Stat(name string) (Timespec, error) {
	if hasPlatformSpecificStat {
		if ts, err := platformSpecficStat(name); err == nil {
			return ts, nil
		}
	}

	fi, err := os.Stat(name)
	if err != nil {
		return nil, err
	}
	return getTimespec(fi), nil
}

// Timespec provides access to file times.
// ChangeTime() panics unless HasChangeTime() is true and
// BirthTime() panics unless HasBirthTime() is true.
type Timespec interface {
	ModTime() time.Time
	AccessTime() time.Time
	ChangeTime() time.Time
	BirthTime() time.Time
	HasChangeTime() bool
	HasBirthTime() bool
}

type atime struct {
	v time.Time
}

func (a atime) AccessTime() time.Time { return a.v }

type ctime struct {
	v time.Time
}

func (ctime) HasChangeTime() bool { return true }

func (c ctime) ChangeTime() time.Time { return c.v }

type mtime struct {
	v time.Time
}

func (m mtime) ModTime() time.Time { return m.v }

type btime struct {
	v time.Time
}

func (btime) HasBirthTime() bool { return true }

func (b btime) BirthTime() time.Time { return b.v }

type noctime struct{}

func (noctime) HasChangeTime() bool { return false }

func (noctime) ChangeTime() time.Time { panic("ctime not available") }

type nobtime struct{}

func (nobtime) HasBirthTime() bool { return false }

func (nobtime) BirthTime() time.Time { panic("birthtime not available") }
