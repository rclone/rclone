//go:build windows

package local

import (
	"context"
	"path/filepath"
	"time"
	_ "unsafe" // use go:linkname

	"github.com/fsnotify/fsnotify"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
)

// Hack to enable recursive watchers in fsnotify, which are available for
// Windows and Linux (although not quite what is needed for Linux here), but
// not yet Mac, hence there is no public interface for this in fsnotify just
// yet. This is currently only needed for, and enabled for, Windows builds.
//
// Setting fsnotify.enableRecurse to true enables recursive handling: paths
// that end with with \... or /... as watched recursively.

//go:linkname enableRecurse github.com/fsnotify/fsnotify.enableRecurse
var enableRecurse bool

// ChangeNotify calls the passed function with a path that has had changes.
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {

	// Create new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fs.Errorf(f, "Failed to create watcher: %s", err)
		return
	}

	// Recursive watch of base directory. This is indicated by appending \...
	// or /... to the path.
	enableRecurse = true
	err = watcher.Add(filepath.Join(f.root, "..."))
	if err != nil {
		fs.Errorf(f, "Failed to start watching %s: %s", f.root, err)
	} else {
		fs.Debugf(f, "Started watching %s", f.root)
	}

	// Files and directories changed in the last poll window, mapped to the
	// time at which notification of the change was received.
	changed := make(map[string]time.Time)

	// Start goroutine to handle filesystem events
	go func() {
		// Polling is imitated by accumulating events between ticks. While
		// notifyFunc() could be called immediately on each filesystem event,
		// accumulating turns out to have some advantages in accurately keeping
		// track of entry types (i.e. file or directory), under the
		// interpretation that the notifications sent at each tick are a diff of
		// the state of the filesystem at that tick compared to the previous. It
		// is also assumed by some tests.
		var ticker *time.Ticker
		var tickerC <-chan time.Time

	loop:
		for {
			select {
			case pollInterval, ok := <-pollIntervalChan:
				// Update ticker
				if !ok {
					if ticker != nil {
						ticker.Stop()
					}
					break loop
				}
				if ticker != nil {
					ticker.Stop()
					ticker, tickerC = nil, nil
				}
				if pollInterval != 0 {
					ticker = time.NewTicker(pollInterval)
					tickerC = ticker.C
				}
			case <-tickerC:
				// Notify for all paths that have changed since the last sync, and
				// which were changed at least 1/10 of a second (1e8 nanoseconds)
				// ago. The lag is for de-duping purposes during long writes, which
				// can consist of multiple write notifications in quick succession.
				cutoff := time.Now().Add(-1e8)
				for entryPath, entryTime := range changed {
					if entryTime.Before(cutoff) {
						notifyFunc(filepath.ToSlash(entryPath), fs.EntryUncertain)
						delete(changed, entryPath)
					}
				}
			case event, ok := <-watcher.Events:
				if !ok {
					break loop
				}
				if event.Has(fsnotify.Create) {
					fs.Debugf(f, "Create: %s", event.Name)
				}
				if event.Has(fsnotify.Remove) {
					fs.Debugf(f, "Remove: %s", event.Name)
				}
				if event.Has(fsnotify.Rename) {
					fs.Debugf(f, "Rename: %s", event.Name)
				}
				if event.Has(fsnotify.Write) {
					fs.Debugf(f, "Write: %s", event.Name)
				}
				if event.Has(fsnotify.Chmod) {
					fs.Debugf(f, "Chmod: %s", event.Name)
				}
				entryPath, _ := filepath.Rel(f.root, event.Name)
				changed[entryPath] = time.Now()

				if event.Has(fsnotify.Create) {
					err = walk.Walk(ctx, f, entryPath, false, -1, func(entryPath string, entries fs.DirEntries, err error) error {
						if err != nil {
							// The entry has already been removed, and we do not know what
							// type it was. It can be ignored, as this means it has been both
							// created and removed since the last tick, which will not change
							// the diff at the next tick.
							fs.Errorf(f, "Failed to walk %s, already removed? %s", entryPath, err)
						}
						for _, d := range entries {
							entryPath := d.Remote()
							changed[entryPath] = time.Now()
						}
						return nil
					})
					if err != nil {
						fs.Errorf(f, "Failed to walk %s, already removed? %s", entryPath, err)
					}
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					break loop
				}
				fs.Errorf(f, "Error: %s", err.Error())
			}
		}

		// Close watcher
		err := watcher.Close()
		if err != nil {
			fs.Errorf(f, "Failed to close watcher: %s", err)
		}
	}()
}
