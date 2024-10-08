package local

import (
	"fmt"
	"math"
	"os"
	"runtime"
	"strconv"
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
			err = setBTime(o.path, btime)
			if err != nil {
				outErr = fmt.Errorf("failed to set birth (creation) time: %w", err)
			}
		}
	}
	uid, hasUID := o.parseMetadataInt(m, "uid", 10)
	gid, hasGID := o.parseMetadataInt(m, "gid", 10)
	if hasUID {
		// FIXME should read UID and GID of current user and only attempt to set it if different
		if !hasGID {
			gid = uid
		}
		if runtime.GOOS == "windows" || runtime.GOOS == "plan9" {
			fs.Debugf(o, "Ignoring request to set ownership %o.%o on this OS", gid, uid)
		} else {
			err = os.Chown(o.path, uid, gid)
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
				err = os.Chmod(o.path, os.FileMode(umode))
				if err != nil {
					outErr = fmt.Errorf("failed to change permissions: %w", err)
				}
			}
		}
	}
	// FIXME not parsing rdev yet
	return outErr
}
