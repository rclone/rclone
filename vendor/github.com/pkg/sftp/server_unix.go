// +build darwin dragonfly freebsd !android,linux netbsd openbsd solaris
// +build cgo

package sftp

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"strconv"
	"syscall"
	"time"
)

func runLsStatt(dirent os.FileInfo, statt *syscall.Stat_t) string {
	// example from openssh sftp server:
	// crw-rw-rw-    1 root     wheel           0 Jul 31 20:52 ttyvd
	// format:
	// {directory / char device / etc}{rwxrwxrwx}  {number of links} owner group size month day [time (this year) | year (otherwise)] name

	typeword := runLsTypeWord(dirent)
	numLinks := statt.Nlink
	uid := statt.Uid
	usr, err := user.LookupId(strconv.Itoa(int(uid)))
	var username string
	if err == nil {
		username = usr.Username
	} else {
		username = fmt.Sprintf("%d", uid)
	}
	gid := statt.Gid
	grp, err := user.LookupGroupId(strconv.Itoa(int(gid)))
	var groupname string
	if err == nil {
		groupname = grp.Name
	} else {
		groupname = fmt.Sprintf("%d", gid)
	}

	mtime := dirent.ModTime()
	monthStr := mtime.Month().String()[0:3]
	day := mtime.Day()
	year := mtime.Year()
	now := time.Now()
	isOld := mtime.Before(now.Add(-time.Hour * 24 * 365 / 2))

	yearOrTime := fmt.Sprintf("%02d:%02d", mtime.Hour(), mtime.Minute())
	if isOld {
		yearOrTime = fmt.Sprintf("%d", year)
	}

	return fmt.Sprintf("%s %4d %-8s %-8s %8d %s %2d %5s %s", typeword, numLinks, username, groupname, dirent.Size(), monthStr, day, yearOrTime, dirent.Name())
}

// ls -l style output for a file, which is in the 'long output' section of a readdir response packet
// this is a very simple (lazy) implementation, just enough to look almost like openssh in a few basic cases
func runLs(dirname string, dirent os.FileInfo) string {
	dsys := dirent.Sys()
	if dsys == nil {
	} else if statt, ok := dsys.(*syscall.Stat_t); !ok {
	} else {
		return runLsStatt(dirent, statt)
	}

	return path.Join(dirname, dirent.Name())
}
