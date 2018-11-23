// Clean the left over test files

// +build go1.11

package main

import (
	"log"
	"regexp"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/list"
	"github.com/ncw/rclone/fs/operations"
)

// MatchTestRemote matches the remote names used for testing (copied
// from fstest/fstest.go so we don't have to import that and get all
// its flags)
var MatchTestRemote = regexp.MustCompile(`^rclone-test-[abcdefghijklmnopqrstuvwxyz0123456789]{24}$`)

// cleanFs runs a single clean fs for left over directories
func cleanFs(remote string) error {
	f, err := fs.NewFs(remote)
	if err != nil {
		return err
	}
	entries, err := list.DirSorted(f, true, "")
	if err != nil {
		return err
	}
	return entries.ForDirError(func(dir fs.Directory) error {
		dirPath := dir.Remote()
		fullPath := remote + dirPath
		if MatchTestRemote.MatchString(dirPath) {
			if *dryRun {
				log.Printf("Not Purging %s - -dry-run", fullPath)
				return nil
			}
			log.Printf("Purging %s", fullPath)
			dir, err := fs.NewFs(fullPath)
			if err != nil {
				return err
			}
			return operations.Purge(dir, "")
		}
		return nil
	})
}

// cleanRemotes cleans the list of remotes passed in
func cleanRemotes(remotes []string) error {
	var lastError error
	for _, remote := range remotes {
		log.Printf("%q - Cleaning", remote)
		err := cleanFs(remote)
		if err != nil {
			lastError = err
			log.Printf("Failed to purge %q: %v", remote, err)
		}
	}
	return lastError
}
