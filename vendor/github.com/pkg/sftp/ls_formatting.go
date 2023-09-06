package sftp

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"strconv"
	"time"

	sshfx "github.com/pkg/sftp/internal/encoding/ssh/filexfer"
)

func lsFormatID(id uint32) string {
	return strconv.FormatUint(uint64(id), 10)
}

type osIDLookup struct{}

func (osIDLookup) Filelist(*Request) (ListerAt, error) {
	return nil, errors.New("unimplemented stub")
}

func (osIDLookup) LookupUserName(uid string) string {
	u, err := user.LookupId(uid)
	if err != nil {
		return uid
	}

	return u.Username
}

func (osIDLookup) LookupGroupName(gid string) string {
	g, err := user.LookupGroupId(gid)
	if err != nil {
		return gid
	}

	return g.Name
}

// runLs formats the FileInfo as per `ls -l` style, which is in the 'longname' field of a SSH_FXP_NAME entry.
// This is a fairly simple implementation, just enough to look close to openssh in simple cases.
func runLs(idLookup NameLookupFileLister, dirent os.FileInfo) string {
	// example from openssh sftp server:
	// crw-rw-rw-    1 root     wheel           0 Jul 31 20:52 ttyvd
	// format:
	// {directory / char device / etc}{rwxrwxrwx}  {number of links} owner group size month day [time (this year) | year (otherwise)] name

	symPerms := sshfx.FileMode(fromFileMode(dirent.Mode())).String()

	var numLinks uint64 = 1
	uid, gid := "0", "0"

	switch sys := dirent.Sys().(type) {
	case *sshfx.Attributes:
		uid = lsFormatID(sys.UID)
		gid = lsFormatID(sys.GID)
	case *FileStat:
		uid = lsFormatID(sys.UID)
		gid = lsFormatID(sys.GID)
	default:
		numLinks, uid, gid = lsLinksUIDGID(dirent)
	}

	if idLookup != nil {
		uid, gid = idLookup.LookupUserName(uid), idLookup.LookupGroupName(gid)
	}

	mtime := dirent.ModTime()
	date := mtime.Format("Jan 2")

	var yearOrTime string
	if mtime.Before(time.Now().AddDate(0, -6, 0)) {
		yearOrTime = mtime.Format("2006")
	} else {
		yearOrTime = mtime.Format("15:04")
	}

	return fmt.Sprintf("%s %4d %-8s %-8s %8d %s %5s %s", symPerms, numLinks, uid, gid, dirent.Size(), date, yearOrTime, dirent.Name())
}
