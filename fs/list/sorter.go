package list

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/lanrat/extsort"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/lib/errcount"
	"golang.org/x/sync/errgroup"
)

// NewObjecter is the minimum facilities we need from the fs.Fs passed into NewSorter.
type NewObjecter interface {
	// NewObject finds the Object at remote.  If it can't be found
	// it returns the error ErrorObjectNotFound.
	NewObject(ctx context.Context, remote string) (fs.Object, error)
}

// Sorter implements an efficient mechanism for sorting list entries.
//
// If there are a large number of entries (above `--list-cutoff`),
// this may be done on disk instead of in memory.
//
// Supply entries with the Add method, call Send at the end to deliver
// the sorted entries and finalise with CleanUp regardless of whether
// you called Add or Send.
//
// Sorted entries are delivered to the callback supplied to NewSorter
// when the Send method is called.
type Sorter struct {
	ctx        context.Context       // context for everything
	ci         *fs.ConfigInfo        // config we are using
	cancel     func()                // cancel all background operations
	mu         sync.Mutex            // protect the below
	f          NewObjecter           // fs that we are listing
	callback   fs.ListRCallback      // where to send the sorted entries to
	entries    fs.DirEntries         // accumulated entries
	keyFn      KeyFn                 // transform an entry into a sort key
	cutoff     int                   // number of entries above which we start extsort
	extSort    bool                  // true if we are ext sorting
	inputChan  chan string           // for sending data to the ext sort
	outputChan <-chan string         // for receiving data from the ext sort
	errChan    <-chan error          // for getting errors from the ext sort
	sorter     *extsort.StringSorter // external string sort
	errs       *errcount.ErrCount    // accumulate errors
}

// KeyFn turns an entry into a sort key
type KeyFn func(entry fs.DirEntry) string

// identityKeyFn maps an entry to its Remote
func identityKeyFn(entry fs.DirEntry) string {
	return entry.Remote()
}

// NewSorter creates a new Sorter with callback for sorted entries to
// be delivered to. keyFn is used to process each entry to get a key
// function, if nil then it will just use entry.Remote()
func NewSorter(ctx context.Context, f NewObjecter, callback fs.ListRCallback, keyFn KeyFn) (*Sorter, error) {
	ci := fs.GetConfig(ctx)
	ctx, cancel := context.WithCancel(ctx)
	if keyFn == nil {
		keyFn = identityKeyFn
	}
	return &Sorter{
		ctx:      ctx,
		ci:       ci,
		cancel:   cancel,
		f:        f,
		callback: callback,
		keyFn:    keyFn,
		cutoff:   ci.ListCutoff,
		errs:     errcount.New(),
	}, nil
}

// Turn a directory entry into a combined key and data for extsort
func (ls *Sorter) entryToKey(entry fs.DirEntry) string {
	// To start with we just use the Remote to recover the object
	// To make more efficient we would serialize the object here
	remote := entry.Remote()
	remote = strings.TrimRight(remote, "/")
	if _, isDir := entry.(fs.Directory); isDir {
		remote += "/"
	}
	key := ls.keyFn(entry) + "\x00" + remote
	return key
}

// Turn an exsort key back into a directory entry
func (ls *Sorter) keyToEntry(ctx context.Context, key string) (entry fs.DirEntry, err error) {
	null := strings.IndexRune(key, '\x00')
	if null < 0 {
		return nil, errors.New("sorter: failed to deserialize: missing null")
	}
	remote := key[null+1:]
	if remote, isDir := strings.CutSuffix(remote, "/"); isDir {
		// Is a directory
		//
		// Note this creates a very minimal directory entry which should be fine for the
		// bucket based remotes this code will be run on.
		entry = fs.NewDir(remote, time.Time{})
	} else {
		obj, err := ls.f.NewObject(ctx, remote)
		if err != nil {
			fs.Errorf(ls.f, "sorter: failed to re-create object %q: %v", remote, err)
			return nil, fmt.Errorf("sorter: failed to re-create object: %w", err)
		}
		entry = obj
	}
	return entry, nil
}

func (ls *Sorter) sendEntriesToExtSort(entries fs.DirEntries) (err error) {
	for _, entry := range entries {
		select {
		case ls.inputChan <- ls.entryToKey(entry):
		case err = <-ls.errChan:
			if err != nil {
				return err
			}
		}
	}
	select {
	case err = <-ls.errChan:
	default:
	}
	return err
}

func (ls *Sorter) startExtSort() (err error) {
	fs.Logf(ls.f, "Switching to on disk sorting as more than %d entries in one directory detected", ls.cutoff)
	ls.inputChan = make(chan string, 100)
	// Options to control the extsort
	opt := extsort.Config{
		NumWorkers:         8,         // small effect
		ChanBuffSize:       1024,      // small effect
		SortedChanBuffSize: 1024,      // makes a lot of difference
		ChunkSize:          32 * 1024, // tuned for 50 char records (UUID sized)
		// Defaults
		// ChunkSize:          int(1e6),	// amount of records to store in each chunk which will be written to disk
		// NumWorkers:         2,		// maximum number of workers to use for parallel sorting
		// ChanBuffSize:       1,		// buffer size for merging chunks
		// SortedChanBuffSize: 10,		// buffer size for passing records to output
		// TempFilesDir:       "",		// empty for use OS default ex: /tmp
	}
	ls.sorter, ls.outputChan, ls.errChan = extsort.Strings(ls.inputChan, &opt)
	go ls.sorter.Sort(ls.ctx)

	// Show we are extsorting now
	ls.extSort = true

	// Send the accumulated entries to the sorter
	fs.Debugf(ls.f, "Sending accumulated directory entries to disk")
	err = ls.sendEntriesToExtSort(ls.entries)
	fs.Debugf(ls.f, "Done sending accumulated directory entries to disk")
	clear(ls.entries)
	ls.entries = nil
	return err
}

// Add entries to the list sorter.
//
// Does not call the callback.
//
// Safe to call from concurrent go routines
func (ls *Sorter) Add(entries fs.DirEntries) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()
	if ls.extSort {
		err := ls.sendEntriesToExtSort(entries)
		if err != nil {
			return err
		}
	} else {
		ls.entries = append(ls.entries, entries...)
		if len(ls.entries) >= ls.cutoff {
			err := ls.startExtSort()
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// Number of entries to batch in list helper
const listHelperBatchSize = 100

// listHelper is used to turn keys into entries concurrently
type listHelper struct {
	ls      *Sorter       // parent
	keys    []string      // keys being built up
	entries fs.DirEntries // entries processed concurrently as a batch
	errs    []error       // errors processed concurrently
}

// NewlistHelper should be with the callback passed in
func (ls *Sorter) newListHelper() *listHelper {
	return &listHelper{
		ls:      ls,
		entries: make(fs.DirEntries, listHelperBatchSize),
		errs:    make([]error, listHelperBatchSize),
	}
}

// send sends the stored entries to the callback if there are >= max
// entries.
func (lh *listHelper) send(max int) (err error) {
	if len(lh.keys) < max {
		return nil
	}

	// Turn this batch into objects in parallel
	g, gCtx := errgroup.WithContext(lh.ls.ctx)
	g.SetLimit(lh.ls.ci.Checkers)
	for i, key := range lh.keys {
		g.Go(func() error {
			lh.entries[i], lh.errs[i] = lh.ls.keyToEntry(gCtx, key)
			return nil
		})
	}
	err = g.Wait()
	if err != nil {
		return err
	}

	// Account errors and collect OK entries
	toSend := lh.entries[:0]
	for i := range lh.keys {
		entry, err := lh.entries[i], lh.errs[i]
		if err != nil {
			lh.ls.errs.Add(err)
		} else if entry != nil {
			toSend = append(toSend, entry)
		}
	}

	// fmt.Println(lh.keys)
	// fmt.Println(toSend)
	err = lh.ls.callback(toSend)

	clear(lh.entries)
	clear(lh.errs)
	lh.keys = lh.keys[:0]
	return err
}

// Add an entry to the stored entries and send them if there are more
// than a certain amount
func (lh *listHelper) Add(key string) error {
	lh.keys = append(lh.keys, key)
	return lh.send(100)
}

// Flush the stored entries (if any) sending them to the callback
func (lh *listHelper) Flush() error {
	return lh.send(1)
}

// Send the sorted entries to the callback.
func (ls *Sorter) Send() (err error) {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	if ls.extSort {
		close(ls.inputChan)

		list := ls.newListHelper()

	outer:
		for {
			select {
			case key, ok := <-ls.outputChan:
				if !ok {
					break outer
				}
				err := list.Add(key)
				if err != nil {
					return err
				}
			case err := <-ls.errChan:
				if err != nil {
					return err
				}
			}
		}
		err = list.Flush()
		if err != nil {
			return err
		}
		return ls.errs.Err("sorter")

	}

	// Sort the directory entries by Remote
	//
	// We use a stable sort here just in case there are
	// duplicates. Assuming the remote delivers the entries in a
	// consistent order, this will give the best user experience
	// in syncing as it will use the first entry for the sync
	// comparison.
	slices.SortStableFunc(ls.entries, func(a, b fs.DirEntry) int {
		return cmp.Compare(ls.keyFn(a), ls.keyFn(b))
	})
	return ls.callback(ls.entries)
}

// CleanUp the Sorter, cleaning up any memory / files.
//
// It is safe and encouraged to call this regardless of whether you
// called Send or not.
//
// This does not call the callback
func (ls *Sorter) CleanUp() {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	ls.cancel()
	clear(ls.entries)
	ls.entries = nil
	ls.extSort = false
}

// SortToChan makes a callback for the Sorter which sends the output
// to the channel provided.
func SortToChan(out chan<- fs.DirEntry) fs.ListRCallback {
	return func(entries fs.DirEntries) error {
		for _, entry := range entries {
			out <- entry
		}
		return nil
	}
}
