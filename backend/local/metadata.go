package local

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
)

const metadataTimeFormat = time.RFC3339Nano

// system metadata keys which this backend owns
//
// not all values supported on all OSes
var systemMetadataInfo = map[string]fs.MetadataHelp{
	"mode": {
		Help:    "File type and mode",
		Type:    "octal, unix style",
		Example: "0100664",
	},
	"uid": {
		Help:    "User ID of owner",
		Type:    "decimal number",
		Example: "500",
	},
	"gid": {
		Help:    "Group ID of owner",
		Type:    "decimal number",
		Example: "500",
	},
	"rdev": {
		Help:    "Device ID (if special file)",
		Type:    "hexadecimal",
		Example: "1abc",
	},
	"atime": {
		Help:    "Time of last access",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999999999Z07:00",
	},
	"mtime": {
		Help:    "Time of last modification",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999999999Z07:00",
	},
	"btime": {
		Help:    "Time of file birth (creation)",
		Type:    "RFC 3339",
		Example: "2006-01-02T15:04:05.999999999Z07:00",
	},
}

// parse a time string from metadata with key
func (o *Object) parseMetadataTime(m fs.Metadata, key string) (t time.Time, ok bool) {
	value, ok := m[key]
	if ok {
		var err error
		t, err = time.Parse(metadataTimeFormat, value)
		if err != nil {
			fs.Debugf(o, "failed to parse metadata %s: %q: %v", key, value, err)
			ok = false
		}
	}
	return t, ok
}

// parse am int from metadata with key and base
func (o *Object) parseMetadataInt(m fs.Metadata, key string, base int) (result int, ok bool) {
	value, ok := m[key]
	if ok {
		var err error
		parsed, err := strconv.ParseInt(value, base, 0)
		if err != nil {
			fs.Debugf(o, "failed to parse metadata %s: %q: %v", key, value, err)
			ok = false
		}
		result = int(parsed)
	}
	return result, ok
}

// Write the metadata into the file
//
// It isn't possible to set the ctime and btime under Unix
func (o *Object) writeMetadataToFile(m fs.Metadata) (outErr error) {
	var err error
	atime, atimeOK := o.parseMetadataTime(m, "atime")
	mtime, mtimeOK := o.parseMetadataTime(m, "mtime")
	btime, btimeOK := o.parseMetadataTime(m, "btime")
	if atimeOK || mtimeOK {
		if atimeOK && !mtimeOK {
			mtime = atime
		}
		if !atimeOK && mtimeOK {
			atime = mtime
		}
		err = o.setTimes(atime, mtime)
		if err != nil {
			outErr = fmt.Errorf("failed to set times: %w", err)
		}
	}
	if haveSetBTime {
		if btimeOK {
			// When translating symlinks, never follow the path. A planted symlink must
			// not redirect the birth-time write out of the root. The NOFOLLOW open is a
			// no-op on a real file or directory
			if o.translatedLink || o.fs.opt.TranslateSymlinks {
				err = lsetBTime(o.path, btime)
			} else {
				err = setBTime(o.path, btime)
			}
			if err != nil {
				outErr = fmt.Errorf("failed to set birth (creation) time: %w", err)
			}
		}
	}
	uid, hasUID := o.parseMetadataInt(m, "uid", 10)
	gid, hasGID := o.parseMetadataInt(m, "gid", 10)
	if hasUID {
		if !hasGID {
			gid = uid
		}
		if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
			fs.Debugf(o, "Ignoring request to set ownership %o.%o on this OS", gid, uid)
		} else if setUID, setGID := ownerToSet(uid, gid); setUID < 0 && setGID < 0 {
			fs.Debugf(o, "Not setting ownership %d.%d: only root can", uid, gid)
		} else {
			uid, gid = setUID, setGID
			if o.translatedLink {
				err = os.Lchown(o.path, uid, gid)
			} else {
				err = o.fs.chown(o.path, uid, gid)
			}
			if err != nil {
				outErr = fmt.Errorf("failed to change ownership: %w", err)
			}
		}
	}
	mode, hasMode := o.parseMetadataInt(m, "mode", 8)
	if hasMode {
		if mode >= 0 {
			umode := uint(mode)
			if umode <= math.MaxUint32 {
				fileMode := modeFromMetadata(uint32(umode))
				// fileMode comes from the source, which may be untrusted, so
				// by default apply only the permission bits and strip the
				// setuid, setgid and sticky bits - except the setgid bit of
				// a directory, which only makes new entries inherit the
				// directory's group.
				if !o.fs.opt.MetadataRestoreSpecial {
					setgidDir := fileMode&os.ModeSetgid != 0 && o.isDirectory()
					fileMode = fileMode.Perm()
					if setgidDir {
						fileMode |= os.ModeSetgid
					}
				}
				if o.translatedLink {
					if haveLChmod {
						err = lChmod(o.path, fileMode)
					} else {
						fs.Debugf(o, "Unable to set mode %v on a symlink on this OS", fileMode)
						err = nil
					}
				} else {
					err = o.fs.chmod(o.path, fileMode)
				}
				if err != nil {
					outErr = fmt.Errorf("failed to change permissions: %w", err)
				}
			}
		}
	}
	// FIXME not parsing rdev yet
	return outErr
}

// processGroups are the supplementary groups of this process
var processGroups = sync.OnceValue(func() []int {
	groups, _ := os.Getgroups()
	return groups
})

// ownerToSet returns the uid and gid to pass to chown for the wanted
// ownership: unchanged for root, else -1 (leave it as it is) for a uid
// other than our own and for a group we are not a member of, which
// only root may set.
func ownerToSet(uid, gid int) (setUID, setGID int) {
	euid := os.Geteuid()
	if euid <= 0 {
		return uid, gid
	}
	if uid != euid {
		uid = -1
	}
	if gid != os.Getegid() && !slices.Contains(processGroups(), gid) {
		gid = -1
	}
	return uid, gid
}

// modeFromMetadata converts the "mode" metadata to an os.FileMode.
//
// On unix the metadata is the st_mode, whose setuid, setgid and sticky
// bits (0o4000, 0o2000, 0o1000) are not the bits os.FileMode uses for
// them; elsewhere it is an os.FileMode, which leaves 0o7000 unused.
func modeFromMetadata(mode uint32) os.FileMode {
	fileMode := os.FileMode(mode &^ 0o7000)
	if mode&0o4000 != 0 {
		fileMode |= os.ModeSetuid
	}
	if mode&0o2000 != 0 {
		fileMode |= os.ModeSetgid
	}
	if mode&0o1000 != 0 {
		fileMode |= os.ModeSticky
	}
	return fileMode
}

// isDirectory reports whether the object on disk is a directory, from
// the file system rather than from the (untrusted) source metadata
func (o *Object) isDirectory() bool {
	if o.translatedLink {
		return false
	}
	fi, err := os.Lstat(o.path)
	return err == nil && fi.IsDir()
}
