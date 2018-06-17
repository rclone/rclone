package sftp

// Methods on the Request object to make working with the Flags bitmasks and
// Attr(ibutes) byte blob easier. Use Pflags() when working with an Open/Write
// request and AttrFlags() and Attributes() when working with SetStat requests.
import "os"

// File Open and Write Flags. Correlate directly with with os.OpenFile flags
// (https://golang.org/pkg/os/#pkg-constants).
type FileOpenFlags struct {
	Read, Write, Append, Creat, Trunc, Excl bool
}

func newFileOpenFlags(flags uint32) FileOpenFlags {
	return FileOpenFlags{
		Read:   flags&ssh_FXF_READ != 0,
		Write:  flags&ssh_FXF_WRITE != 0,
		Append: flags&ssh_FXF_APPEND != 0,
		Creat:  flags&ssh_FXF_CREAT != 0,
		Trunc:  flags&ssh_FXF_TRUNC != 0,
		Excl:   flags&ssh_FXF_EXCL != 0,
	}
}

// Pflags converts the bitmap/uint32 from SFTP Open packet pflag values,
// into a FileOpenFlags struct with booleans set for flags set in bitmap.
func (r *Request) Pflags() FileOpenFlags {
	return newFileOpenFlags(r.Flags)
}

// Flags that indicate whether SFTP file attributes were passed. When a flag is
// true the corresponding attribute should be available from the FileStat
// object returned by Attributes method. Used with SetStat.
type FileAttrFlags struct {
	Size, UidGid, Permissions, Acmodtime bool
}

func newFileAttrFlags(flags uint32) FileAttrFlags {
	return FileAttrFlags{
		Size:        (flags & ssh_FILEXFER_ATTR_SIZE) != 0,
		UidGid:      (flags & ssh_FILEXFER_ATTR_UIDGID) != 0,
		Permissions: (flags & ssh_FILEXFER_ATTR_PERMISSIONS) != 0,
		Acmodtime:   (flags & ssh_FILEXFER_ATTR_ACMODTIME) != 0,
	}
}

// FileAttrFlags returns a FileAttrFlags boolean struct based on the
// bitmap/uint32 file attribute flags from the SFTP packaet.
func (r *Request) AttrFlags() FileAttrFlags {
	return newFileAttrFlags(r.Flags)
}

// FileMode returns the Mode SFTP file attributes wrapped as os.FileMode
func (a FileStat) FileMode() os.FileMode {
	return os.FileMode(a.Mode)
}

// Attributres parses file attributes byte blob and return them in a
// FileStat object.
func (r *Request) Attributes() *FileStat {
	fs, _ := getFileStat(r.Flags, r.Attrs)
	return fs
}
