// Clean the left over test files

// +build go1.11

package main

import (
	"context"
	"log"
	"regexp"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/list"
	"github.com/rclone/rclone/fs/operations"
)

// MatchTestRemote matches the remote names used for testing (copied
// from fstest/fstest.go so we don't have to import that and get all
// its flags)
var MatchTestRemote = regexp.MustCompile(`^rclone-test-[abcdefghijklmnopqrstuvwxyz0123456789]{24}(_segments)?$`)

// cleanFs runs a single clean fs for left over directories
func cleanFs(remote string) error {
	f, err := fs.NewFs(remote)
	if err != nil {
		return err
	}
	entries, err := list.DirSorted(context.Background(), f, true, "")
	if err != nil {
		return err
	}
	var lastErr error
	err = entries.ForDirError(func(dir fs.Directory) error {
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
				err = errors.Wrap(err, "NewFs failed")
				lastErr = err
				fs.Errorf(fullPath, "%v", err)
				return nil
			}
			err = operations.Purge(context.Background(), dir, "")
			if err != nil {
				err = errors.Wrap(err, "Purge failed")
				lastErr = err
				fs.Errorf(dir, "%v", err)
				return nil
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	return lastErr
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
