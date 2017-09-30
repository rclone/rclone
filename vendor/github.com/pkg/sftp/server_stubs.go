// +build !cgo,!plan9 windows android

package sftp

import (
	"os"
	"time"
	"fmt"
)

func runLs(dirname string, dirent os.FileInfo) string {
	typeword := runLsTypeWord(dirent)
	numLinks := 1
	if dirent.IsDir() {
		numLinks = 0
	}
	username := "root"
	groupname := "root"
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
