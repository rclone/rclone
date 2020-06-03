// This keeps track of the files which need to be written back

package vfscache

import (
	"container/heap"
	"context"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

const (
	maxUploadDelay = 5 * time.Minute // max delay betwen upload attempts
)

// writeBack keeps track of the items which need to be written back to the disk at some point
type writeBack struct {
	mu      sync.Mutex
	items   writeBackItems           // priority queue of *writeBackItem - writeBackItems are in here while awaiting transfer only
	lookup  map[*Item]*writeBackItem // for getting a *writeBackItem from a *Item - writeBackItems are in here until cancelled
	opt     *vfscommon.Options       // VFS options
	timer   *time.Timer              // next scheduled time for the uploader
	kick    chan struct{}            // send on this channel to wake up the uploader
	uploads int                      // number of uploads in progress
}

// make a new writeBack
//
// cancel the context to stop the background goroutine
func newWriteBack(ctx context.Context, opt *vfscommon.Options) *writeBack {
	wb := &writeBack{
		items:  writeBackItems{},
		lookup: make(map[*Item]*writeBackItem),
		opt:    opt,
		timer:  time.NewTimer(time.Second),
		kick:   make(chan struct{}, 1),
	}
	wb.timer.Stop()
	heap.Init(&wb.items)
	go wb.uploader(ctx)
	return wb
}

// writeBackItem stores an Item awaiting writeback
//
// These are stored on the items heap when awaiting transfer but
// removed from the items heap when transferring. They remain in the
// lookup map until cancelled.
//
// writeBack.mu must be held to manipulate this
type writeBackItem struct {
	name      string             // name of the item so we don't have to read it from item
	index     int                // index into the priority queue for update
	item      *Item              // Item that needs writeback
	expiry    time.Time          // When this expires we will write it back
	uploading bool               // If we are uploading the item
	cancel    context.CancelFunc // To cancel the upload with
	done      chan struct{}      // closed when the cancellation completes
	storeFn   StoreFn            // To write the object back with
	tries     int                // number of times we have tried to upload
	delay     time.Duration      // delay between upload attempts
}

// A writeBackItems implements a priority queue by implementing
// heap.Interface and holds writeBackItems.
type writeBackItems []*writeBackItem

func (ws writeBackItems) Len() int { return len(ws) }

func (ws writeBackItems) Less(i, j int) bool {
	return ws[i].expiry.Sub(ws[j].expiry) < 0
}

func (ws writeBackItems) Swap(i, j int) {
	ws[i], ws[j] = ws[j], ws[i]
	ws[i].index = i
	ws[j].index = j
}

func (ws *writeBackItems) Push(x interface{}) {
	n := len(*ws)
	item := x.(*writeBackItem)
	item.index = n
	*ws = append(*ws, item)
}

func (ws *writeBackItems) Pop() interface{} {
	old := *ws
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*ws = old[0 : n-1]
	return item
}

// update modifies the expiry of an Item in the queue.
//
// call with lock held
func (ws *writeBackItems) _update(item *writeBackItem, expiry time.Time) {
	item.expiry = expiry
	heap.Fix(ws, item.index)
}

// return a new expiry time based from now until the WriteBack timeout
//
// call with lock held
func (wb *writeBack) _newExpiry() time.Time {
	expiry := time.Now()
	if wb.opt.WriteBack > 0 {
		expiry = expiry.Add(wb.opt.WriteBack)
	}
	return expiry
}

// make a new writeBackItem
//
// call with the lock held
func (wb *writeBack) _newItem(item *Item, name string) *writeBackItem {
	wbItem := &writeBackItem{
		name:   name,
		item:   item,
		expiry: wb._newExpiry(),
		delay:  wb.opt.WriteBack,
	}
	wb._addItem(wbItem)
	wb._pushItem(wbItem)
	return wbItem
}

// add a writeBackItem to the lookup map
//
// call with the lock held
func (wb *writeBack) _addItem(wbItem *writeBackItem) {
	wb.lookup[wbItem.item] = wbItem
}

// delete a writeBackItem from the lookup map
//
// call with the lock held
func (wb *writeBack) _delItem(wbItem *writeBackItem) {
	delete(wb.lookup, wbItem.item)
}

// pop a writeBackItem from the items heap
//
// call with the lock held
func (wb *writeBack) _popItem() (wbItem *writeBackItem) {
	return heap.Pop(&wb.items).(*writeBackItem)
}

// push a writeBackItem onto the items heap
//
// call with the lock held
func (wb *writeBack) _pushItem(wbItem *writeBackItem) {
	heap.Push(&wb.items, wbItem)
}

// remove a writeBackItem from the items heap
//
// call with the lock held
func (wb *writeBack) _removeItem(wbItem *writeBackItem) {
	heap.Remove(&wb.items, wbItem.index)
}

// peek the oldest writeBackItem - may be nil
//
// call with the lock held
func (wb *writeBack) _peekItem() (wbItem *writeBackItem) {
	if len(wb.items) == 0 {
		return nil
	}
	return wb.items[0]
}

// reset the timer which runs the expiries
func (wb *writeBack) _resetTimer() {
	wbItem := wb._peekItem()
	if wbItem == nil {
		wb.timer.Stop()
	} else {
		dt := time.Until(wbItem.expiry)
		if dt < 0 {
			dt = 0
		}
		wb.timer.Reset(dt)
	}
}

// add adds an item to the writeback queue or resets its timer if it
// is already there
func (wb *writeBack) add(item *Item, name string, storeFn StoreFn) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wbItem, ok := wb.lookup[item]
	if !ok {
		wbItem = wb._newItem(item, name)
	} else {
		if wbItem.uploading {
			// We are uploading already so cancel the upload
			wb._cancelUpload(wbItem)
		}
		// Kick the timer on
		wb.items._update(wbItem, wb._newExpiry())
	}
	wbItem.storeFn = storeFn
	wb._resetTimer()
}

// cancel a writeback if there is one
func (wb *writeBack) cancel(item *Item) (found bool) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	wbItem, found := wb.lookup[item]
	if found {
		fs.Debugf(wbItem.name, "vfs cache: cancelling writeback")
		if wbItem.uploading {
			// We are uploading already so cancel the upload
			wb._cancelUpload(wbItem)
		} else {
			// Remove the item from the heap
			wb._removeItem(wbItem)
		}
		// Remove the item from the lookup map
		wb._delItem(wbItem)
	}
	wb._resetTimer()
	return found
}

// kick the upload checker
//
// This should be called at the end of uploads just in case we had to
// pause uploades because max items was exceeded
//
// call with the lock held
func (wb *writeBack) _kickUploader() {
	select {
	case wb.kick <- struct{}{}:
	default:
	}
}

// upload the item - called as a goroutine
func (wb *writeBack) upload(ctx context.Context, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	item := wbItem.item
	wbItem.tries++

	wb.mu.Unlock()
	err := item.store(ctx, wbItem.storeFn)
	wb.mu.Lock()

	wbItem.cancel() // cancel context to release resources since store done
	if wbItem.uploading {
		wbItem.uploading = false
		wb.uploads--
	}

	if err != nil {
		// FIXME should this have a max number of transfer attempts?
		wbItem.delay *= 2
		if wbItem.delay > maxUploadDelay {
			wbItem.delay = maxUploadDelay
		}
		fs.Errorf(wbItem.name, "vfs cache: failed to upload try #%d, will retry in %v: %v", wbItem.tries, wbItem.delay, err)
		// push the item back on the queue for retry
		wb._pushItem(wbItem)
		wb.items._update(wbItem, time.Now().Add(wbItem.delay))
	} else {
		fs.Infof(wbItem.name, "vfs cache: upload succeeded try #%d", wbItem.tries)
		// show that we are done with the item
		wb._delItem(wbItem)
	}
	wb._kickUploader()
	close(wbItem.done)
}

// cancel the upload
//
// call with lock held
func (wb *writeBack) _cancelUpload(wbItem *writeBackItem) {
	if !wbItem.uploading {
		return
	}
	fs.Debugf(wbItem.name, "vfs cache: cancelling upload")
	if wbItem.cancel != nil {
		// Cancel the upload - this may or may not be effective
		wbItem.cancel()
		// wait for the uploader to finish
		<-wbItem.done
	}
	if wbItem.uploading {
		wbItem.uploading = false
		wb.uploads--
	}
	// uploading items are not on the heap so add them back
	wb._pushItem(wbItem)
}

// this uploads as many items as possible
func (wb *writeBack) processItems(ctx context.Context) {
	wb.mu.Lock()
	defer wb.mu.Unlock()

	resetTimer := false
	for wbItem := wb._peekItem(); wbItem != nil && time.Until(wbItem.expiry) <= 0; wbItem = wb._peekItem() {
		// If reached transfer limit don't restart the timer
		if wb.uploads >= fs.Config.Transfers {
			fs.Debugf(wbItem.name, "vfs cache: delaying writeback as --transfers exceeded")
			resetTimer = false
			break
		}
		resetTimer = true
		// Pop the item, mark as uploading and start the uploader
		wbItem = wb._popItem()
		wbItem.uploading = true
		wb.uploads++
		newCtx, cancel := context.WithCancel(ctx)
		wbItem.cancel = cancel
		wbItem.done = make(chan struct{})
		go wb.upload(newCtx, wbItem)
	}

	if resetTimer {
		wb._resetTimer()
	}
}

// Looks for items which need writing back and write them back until
// the context is cancelled
func (wb *writeBack) uploader(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			wb.timer.Stop()
			return
		case <-wb.timer.C:
			wb.processItems(ctx)
		case <-wb.kick:
			wb.processItems(ctx)
		}
	}
}

// return the number of uploads in progress
func (wb *writeBack) getStats() (uploadsInProgress, uploadsQueued int) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	return wb.uploads, len(wb.items)
}
