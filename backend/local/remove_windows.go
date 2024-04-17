//go:build windows

package local

import (
	"os"
	"time"

	"github.com/rclone/rclone/fs"
	"golang.org/x/sys/windows"
)

// Removes name, retrying on a sharing violation
func remove(name string) (err error) {
	const maxTries = 10
	var sleepTime = 1 * time.Millisecond
	for i := 0; i < maxTries; i++ {
		err = os.Remove(name)
		if err == nil {
			break
		}
		pathErr, ok := err.(*os.PathError)
		if !ok {
			break
		}
		if pathErr.Err != windows.ERROR_SHARING_VIOLATION {
			break
		}
		fs.Logf(name, "Remove detected sharing violation - retry %d/%d sleeping %v", i+1, maxTries, sleepTime)
		time.Sleep(sleepTime)
		sleepTime <<= 1
	}
	return err
}
