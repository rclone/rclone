package sftp

// Methods on the Request object to make working with the Flags bitmasks and
// Attr(ibutes) byte blob easier. Use Pflags() when working with an Open/Write
// request and AttrFlags() and Attributes() when working with SetStat requests.

import "os"

// Open packet pflags
type pflags struct {
	Read, Write, Append, Creat, Trunc, Excl bool
}

// testable constructor
func newPflags(flags uint32) pflags {
	return pflags{
		Read:   flags&ssh_FXF_READ != 0,
		Write:  flags&ssh_FXF_WRITE != 0,
		Append: flags&ssh_FXF_APPEND != 0,
		Creat:  flags&ssh_FXF_CREAT != 0,
		Trunc:  flags&ssh_FXF_TRUNC != 0,
		Excl:   flags&ssh_FXF_EXCL != 0,
	}
}

// Check bitmap/uint32 for Open packet pflag values
func (r *Request) Pflags() pflags {
	return newPflags(r.Flags)
}

// File attribute flags
type aflags struct {
	Size, UidGid, Permissions, Acmodtime bool
}

// testable constructor
func newAflags(flags uint32) aflags {
	return aflags{
		Size:        (flags & ssh_FILEXFER_ATTR_SIZE) != 0,
		UidGid:      (flags & ssh_FILEXFER_ATTR_UIDGID) != 0,
		Permissions: (flags & ssh_FILEXFER_ATTR_PERMISSIONS) != 0,
		Acmodtime:   (flags & ssh_FILEXFER_ATTR_ACMODTIME) != 0,
	}
}

// Check bitmap/uint32 for file attribute flags
func (r *Request) AttrFlags(flags uint32) aflags {
	return newAflags(r.Flags)
}

// File attributes
type fileattrs FileStat

// Return Mode wrapped in os.FileMode
func (a fileattrs) FileMode() os.FileMode {
	return os.FileMode(a.Mode)
}

// Parse file attributes byte blob and return them in object
func (r *Request) Attributes() fileattrs {
	fa, _ := getFileStat(r.Flags, r.Attrs)
	return fileattrs(*fa)
}
