//go:build !windows

package local

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/moby/sys/mountinfo"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/walk"
)

// ChangeNotify calls the passed function with a path that has had changes.
// Close the returned channel to stop being notified.
func (f *Fs) ChangeNotify(ctx context.Context, notifyFunc func(string, fs.EntryType), pollIntervalChan <-chan time.Duration) {
	// Will not work with an NFS mounted filesystem, error in this case
	infos, err := mountinfo.GetMounts(mountinfo.ParentsFilter(f.root))
	if err == nil {
		for i := 0; i < len(infos); i++ {
			if infos[i].FSType == "nfs" {
				fs.Error(f, "ChangeNotify does not support NFS mounts")
				return
			}
		}
	}

	// Create new watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fs.Errorf(f, "Failed to create watcher: %s", err)
		return
	}

	// Files and directories changed in the last poll window, mapped to the
	// time at which notification of the change was received.
	changed := make(map[string]time.Time)

	// Channel to handle new paths. Buffered ensures filesystem events keep
	// being consumed.
	watchChan := make(chan string)

	// Channel to synchronize with the watch goroutine
	replyChan := make(chan bool)

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

				if event.Has(fsnotify.Create) {
					fs.Debugf(f, "Create: %s", event.Name)
					watchChan <- event.Name
					<-replyChan // implies mutex on 'changed'
				} else {
					entryPath, _ := filepath.Rel(f.root, event.Name)
					changed[entryPath] = time.Now()

					// Internally, fsnotify stops watching directories that are removed
					// or renamed, so it is not necessary to make updates to the watch
					// list.
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					break loop
				}
				fs.Errorf(f, "Error: %s", err.Error())
			}
		}

		// Close channels
		close(watchChan)
		close(replyChan)

		// Close watcher
		err := watcher.Close()
		if err != nil {
			fs.Errorf(f, "Failed to close watcher: %s", err)
		}
	}()

	// Start goroutine to establish watchers
	go func() {
		for {
			path, ok := <-watchChan
			if !ok {
				break
			}

			// Is this the initial watch?
			initial := path == f.root

			// Determine entry path
			entryPath := ""
			if !initial {
				entryPath, err = filepath.Rel(f.root, path)
				if err != nil {
					// Not in this remote
					replyChan <- true
					continue
				}
			}

			// Determine entry type
			entryType := fs.EntryObject
			if initial {
				// Known to be a directory, but also cannot Lstat() some mounts
				entryType = fs.EntryDirectory
			} else {
				info, err := os.Lstat(path)
				if err != nil {
					fs.Errorf(f, "Failed to stat %s, already removed? %s", path, err)
					replyChan <- true
					continue
				} else if info.IsDir() {
					entryType = fs.EntryDirectory
				}
				changed[entryPath] = time.Now()
			}

			if entryType == fs.EntryDirectory {
				// Recursively watch the directory
				err := watcher.Add(path)
				if err != nil {
					fs.Errorf(f, "Failed to start watching %s, already removed? %s", path, err)
				} else {
					fs.Logf(f, "Started watching %s", path)
				}
				err = walk.Walk(ctx, f, entryPath, false, -1, func(entryPath string, entries fs.DirEntries, err error) error {
					if err != nil {
						// The entry has already been removed, and we do not know what
						// type it was. It can be ignored, as this means it has been both
						// created and removed since the last tick, which will not change
						// the diff at the next tick.
						fs.Errorf(f, "Failed to walk %s, already removed? %s", path, err)
					}
					for _, d := range entries {
						entryPath := d.Remote()
						path := filepath.Join(f.root, entryPath)
						info, err := os.Lstat(path)
						if err != nil {
							fs.Errorf(f, "Failed to stat %s, already removed? %s", path, err)
							continue
						}
						if !initial {
							changed[entryPath] = time.Now()
						}
						if info.IsDir() {
							// Watch the directory.
							//
							// Establishing a watch on a directory before listing its
							// contents ensures that no entries are missed and all changes
							// are notified, even for entries created or modified while
							// the watch is being established.
							//
							// An entry may be created between establishing the watch on
							// the directory and listing the directory. In this case it is
							// marked as changed both by this walk and the subsequent
							// handling of the associated filesystem event. Because
							// changes are accumulated up to the next tick, however, only
							// a single notification is sent at the next tick.
							//
							// If an entry exists when the walk begins, but is removed
							// before the walk reaches it, it is as though that entry
							// never existed. But as both occur since the last tick, this
							// does not affect the diff at the next tick.
							err := watcher.Add(path)
							if err != nil {
								fs.Errorf(f, "Failed to start watching %s, already removed? %s", entryPath, err)
							} else {
								fs.Logf(f, "Started watching %s", entryPath)
							}
						}
					}
					return nil
				})
				if err != nil {
					fs.Errorf(f, "Failed to walk %s, already removed? %s", entryPath, err)
				}
			}
			replyChan <- true
		}
	}()

	// Recursively watch all subdirectories from the root
	watchChan <- f.root

	// Wait until initial watch is established before returning
	<-replyChan
}
