package filexfer

// FileMode represents a file’s mode and permission bits.
// The bits are defined according to POSIX standards,
// and may not apply to the OS being built for.
type FileMode uint32

// Permission flags, defined here to avoid potential inconsistencies in individual OS implementations.
const (
	ModePerm       FileMode = 0o0777 // S_IRWXU | S_IRWXG | S_IRWXO
	ModeUserRead   FileMode = 0o0400 // S_IRUSR
	ModeUserWrite  FileMode = 0o0200 // S_IWUSR
	ModeUserExec   FileMode = 0o0100 // S_IXUSR
	ModeGroupRead  FileMode = 0o0040 // S_IRGRP
	ModeGroupWrite FileMode = 0o0020 // S_IWGRP
	ModeGroupExec  FileMode = 0o0010 // S_IXGRP
	ModeOtherRead  FileMode = 0o0004 // S_IROTH
	ModeOtherWrite FileMode = 0o0002 // S_IWOTH
	ModeOtherExec  FileMode = 0o0001 // S_IXOTH

	ModeSetUID FileMode = 0o4000 // S_ISUID
	ModeSetGID FileMode = 0o2000 // S_ISGID
	ModeSticky FileMode = 0o1000 // S_ISVTX

	ModeType       FileMode = 0xF000 // S_IFMT
	ModeNamedPipe  FileMode = 0x1000 // S_IFIFO
	ModeCharDevice FileMode = 0x2000 // S_IFCHR
	ModeDir        FileMode = 0x4000 // S_IFDIR
	ModeDevice     FileMode = 0x6000 // S_IFBLK
	ModeRegular    FileMode = 0x8000 // S_IFREG
	ModeSymlink    FileMode = 0xA000 // S_IFLNK
	ModeSocket     FileMode = 0xC000 // S_IFSOCK
)

// IsDir reports whether m describes a directory.
// That is, it tests for m.Type() == ModeDir.
func (m FileMode) IsDir() bool {
	return (m & ModeType) == ModeDir
}

// IsRegular reports whether m describes a regular file.
// That is, it tests for m.Type() == ModeRegular
func (m FileMode) IsRegular() bool {
	return (m & ModeType) == ModeRegular
}

// Perm returns the POSIX permission bits in m (m & ModePerm).
func (m FileMode) Perm() FileMode {
	return (m & ModePerm)
}

// Type returns the type bits in m (m & ModeType).
func (m FileMode) Type() FileMode {
	return (m & ModeType)
}

// String returns a `-rwxrwxrwx` style string representing the `ls -l` POSIX permissions string.
func (m FileMode) String() string {
	var buf [10]byte

	switch m.Type() {
	case ModeRegular:
		buf[0] = '-'
	case ModeDir:
		buf[0] = 'd'
	case ModeSymlink:
		buf[0] = 'l'
	case ModeDevice:
		buf[0] = 'b'
	case ModeCharDevice:
		buf[0] = 'c'
	case ModeNamedPipe:
		buf[0] = 'p'
	case ModeSocket:
		buf[0] = 's'
	default:
		buf[0] = '?'
	}

	const rwx = "rwxrwxrwx"
	for i, c := range rwx {
		if m&(1<<uint(9-1-i)) != 0 {
			buf[i+1] = byte(c)
		} else {
			buf[i+1] = '-'
		}
	}

	if m&ModeSetUID != 0 {
		if buf[3] == 'x' {
			buf[3] = 's'
		} else {
			buf[3] = 'S'
		}
	}

	if m&ModeSetGID != 0 {
		if buf[6] == 'x' {
			buf[6] = 's'
		} else {
			buf[6] = 'S'
		}
	}

	if m&ModeSticky != 0 {
		if buf[9] == 'x' {
			buf[9] = 't'
		} else {
			buf[9] = 'T'
		}
	}

	return string(buf[:])
}
