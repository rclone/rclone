package vfscache

import (
	"container/heap"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
)

func newTestWriteBack(t *testing.T) (wb *writeBack, cancel func()) {
	ctx, cancel := context.WithCancel(context.Background())
	opt := vfscommon.DefaultOpt
	opt.WriteBack = 100 * time.Millisecond
	wb = newWriteBack(ctx, &opt)
	return wb, cancel
}

// string for debugging - make a copy and pop the items out in order
func (ws writeBackItems) string(t *testing.T) string {
	// check indexes OK first
	for i := range ws {
		assert.Equal(t, i, ws[i].index, ws[i].name)
	}
	wsCopy := make(writeBackItems, len(ws))
	// deep copy the elements
	for i := range wsCopy {
		item := *ws[i]
		wsCopy[i] = &item
	}
	// print them
	var out []string
	for wsCopy.Len() > 0 {
		out = append(out, heap.Pop(&wsCopy).(*writeBackItem).name)
	}
	return strings.Join(out, ",")
}

func TestWriteBackItems(t *testing.T) {
	// Test the items heap behaves properly
	now := time.Now()
	wbItem1 := writeBackItem{name: "one", expiry: now.Add(1 * time.Second)}
	wbItem2 := writeBackItem{name: "two", expiry: now.Add(2 * time.Second)}
	wbItem3 := writeBackItem{name: "three", expiry: now.Add(4 * time.Second)}

	ws := writeBackItems{}

	heap.Init(&ws)
	assert.Equal(t, "", ws.string(t))
	heap.Push(&ws, &wbItem2)
	assert.Equal(t, "two", ws.string(t))
	heap.Push(&ws, &wbItem3)
	assert.Equal(t, "two,three", ws.string(t))
	heap.Push(&ws, &wbItem1)
	assert.Equal(t, "one,two,three", ws.string(t))

	ws._update(&wbItem1, now.Add(3*time.Second))
	assert.Equal(t, "two,one,three", ws.string(t))

	ws._update(&wbItem1, now.Add(5*time.Second))
	assert.Equal(t, "two,three,one", ws.string(t))
}

func checkOnHeap(t *testing.T, wb *writeBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.True(t, wbItem.onHeap)
	for i := range wb.items {
		if wb.items[i] == wbItem {
			return
		}
	}
	assert.Failf(t, "expecting %q on heap", wbItem.name)
}

func checkNotOnHeap(t *testing.T, wb *writeBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.False(t, wbItem.onHeap)
	for i := range wb.items {
		if wb.items[i] == wbItem {
			t.Errorf("not expecting %q on heap", wbItem.name)
		}
	}
}

func checkInLookup(t *testing.T, wb *writeBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.Equal(t, wbItem, wb.lookup[wbItem.item])
}

func checkNotInLookup(t *testing.T, wb *writeBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.Nil(t, wb.lookup[wbItem.item])
}

func TestWriteBackItemCRUD(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()
	ws := &wb.items
	item1, item2, item3 := &Item{}, &Item{}, &Item{}

	// _peekItem empty
	assert.Nil(t, wb._peekItem())

	wbItem1 := wb._newItem(item1, "one")
	checkOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)

	wbItem2 := wb._newItem(item2, "two")
	checkOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)

	wbItem3 := wb._newItem(item3, "three")
	checkOnHeap(t, wb, wbItem3)
	checkInLookup(t, wb, wbItem3)

	assert.Equal(t, "one,two,three", ws.string(t))

	// _delItem
	wb._delItem(wbItem2)
	checkOnHeap(t, wb, wbItem2)
	checkNotInLookup(t, wb, wbItem2)

	// _addItem
	wb._addItem(wbItem2)
	checkOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)

	// _popItem
	assert.True(t, wbItem1.onHeap)
	poppedWbItem := wb._popItem()
	assert.Equal(t, wbItem1, poppedWbItem)
	checkNotOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)
	assert.Equal(t, "two,three", ws.string(t))

	// _pushItem
	wb._pushItem(wbItem1)
	checkOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)
	assert.Equal(t, "one,two,three", ws.string(t))
	// push twice
	wb._pushItem(wbItem1)
	assert.Equal(t, "one,two,three", ws.string(t))

	// _peekItem
	assert.Equal(t, wbItem1, wb._peekItem())

	// _removeItem
	assert.Equal(t, "one,two,three", ws.string(t))
	wb._removeItem(wbItem2)
	checkNotOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.Equal(t, "one,three", ws.string(t))
	// remove twice
	wb._removeItem(wbItem2)
	checkNotOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.Equal(t, "one,three", ws.string(t))
}

func assertTimerRunning(t *testing.T, wb *writeBack, running bool) {
	wb.mu.Lock()
	if running {
		assert.NotEqual(t, time.Time{}, wb.expiry)
		assert.NotNil(t, wb.timer)
	} else {
		assert.Equal(t, time.Time{}, wb.expiry)
		assert.Nil(t, wb.timer)
	}
	wb.mu.Unlock()
}

func TestWriteBackResetTimer(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	// Reset the timer on an empty queue
	wb._resetTimer()

	// Check timer is stopped
	assertTimerRunning(t, wb, false)

	_ = wb._newItem(&Item{}, "three")

	// Reset the timer on an queue with stuff
	wb._resetTimer()

	// Check timer is not stopped
	assertTimerRunning(t, wb, true)
}

// A "transfer" for testing
type putItem struct {
	wg        sync.WaitGroup
	mu        sync.Mutex
	t         *testing.T
	started   chan struct{}
	errChan   chan error
	running   bool
	cancelled bool
	called    bool
}

func newPutItem(t *testing.T) *putItem {
	return &putItem{
		t:       t,
		started: make(chan struct{}, 1),
	}
}

// put the object as per putFn interface
func (pi *putItem) put(ctx context.Context) (err error) {
	pi.wg.Add(1)
	defer pi.wg.Done()

	pi.mu.Lock()
	pi.called = true
	if pi.running {
		assert.Fail(pi.t, "upload already running")
	}
	pi.running = true
	pi.errChan = make(chan error, 1)
	pi.mu.Unlock()

	pi.started <- struct{}{}

	cancelled := false
	select {
	case err = <-pi.errChan:
	case <-ctx.Done():
		err = ctx.Err()
		cancelled = true
	}

	pi.mu.Lock()
	pi.running = false
	pi.cancelled = cancelled
	pi.mu.Unlock()

	return err
}

// finish the "transfer" with the error passed in
func (pi *putItem) finish(err error) {
	pi.mu.Lock()
	if !pi.running {
		assert.Fail(pi.t, "upload not running")
	}
	pi.mu.Unlock()

	pi.errChan <- err
	pi.wg.Wait()
}

func waitUntilNoTransfers(t *testing.T, wb *writeBack) {
	for i := 0; i < 100; i++ {
		wb.mu.Lock()
		uploads := wb.uploads
		wb.mu.Unlock()
		if uploads == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Errorf("failed to wait for transfer to finish")
}

// This is the happy path with everything working
func TestWriteBackAddOK(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wbItem := wb.add(item, "one", true, pi.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.items.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)
}

// Now test the upload failing and being retried
func TestWriteBackAddFailRetry(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wbItem := wb.add(item, "one", true, pi.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.items.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	pi.finish(errors.New("transfer failed BOOM"))
	waitUntilNoTransfers(t, wb)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// check the retry
	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)
}

// Now test the upload being cancelled by another upload being added
func TestWriteBackAddUpdate(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wbItem := wb.add(item, "one", true, pi.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.items.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// Now the upload has started add another one

	pi2 := newPutItem(t)
	wbItem2 := wb.add(item, "one", true, pi2.put)
	assert.Equal(t, wbItem, wbItem2)
	checkOnHeap(t, wb, wbItem) // object awaiting writeback time
	checkInLookup(t, wb, wbItem)

	// check the previous transfer was cancelled
	assert.True(t, pi.cancelled)

	// check the retry
	<-pi2.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	pi2.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)
}

// Now test the upload being not cancelled by another upload being added
func TestWriteBackAddUpdateNotModified(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wbItem := wb.add(item, "one", false, pi.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.items.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// Now the upload has started add another one

	pi2 := newPutItem(t)
	wbItem2 := wb.add(item, "one", false, pi2.put)
	assert.Equal(t, wbItem, wbItem2)
	checkNotOnHeap(t, wb, wbItem) // object still being transfered
	checkInLookup(t, wb, wbItem)

	// Because modified was false above this should not cancel the
	// transfer
	assert.False(t, pi.cancelled)

	// wait for original transfer to finish
	pi.finish(nil)
	waitUntilNoTransfers(t, wb)
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)

	assert.False(t, pi2.called)
}

// Now test the upload being not cancelled by another upload being
// added because the upload hasn't started yet
func TestWriteBackAddUpdateNotStarted(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wbItem := wb.add(item, "one", true, pi.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.items.string(t))

	// Immediately add another upload before the first has started

	pi2 := newPutItem(t)
	wbItem2 := wb.add(item, "one", true, pi2.put)
	assert.Equal(t, wbItem, wbItem2)
	checkOnHeap(t, wb, wbItem) // object still awaiting transfer
	checkInLookup(t, wb, wbItem)

	// Wait for the upload to start
	<-pi2.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// Because modified was false above this should not cancel the
	// transfer
	assert.False(t, pi.cancelled)

	// wait for new transfer to finish
	pi2.finish(nil)
	waitUntilNoTransfers(t, wb)
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)

	assert.False(t, pi.called)
}

func TestWriteBackGetStats(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}
	pi := newPutItem(t)

	wb.add(item, "one", true, pi.put)

	inProgress, queued := wb.getStats()
	assert.Equal(t, queued, 1)
	assert.Equal(t, inProgress, 0)

	<-pi.started

	inProgress, queued = wb.getStats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 1)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)

	inProgress, queued = wb.getStats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 0)

}

// Test queuing more than fs.Config.Transfers
func TestWriteBackMaxQueue(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	maxTransfers := fs.Config.Transfers
	toTransfer := maxTransfers + 2

	// put toTransfer things in the queue
	pis := []*putItem{}
	for i := 0; i < toTransfer; i++ {
		pi := newPutItem(t)
		pis = append(pis, pi)
		wb.add(&Item{}, fmt.Sprintf("number%d", 1), true, pi.put)
	}

	inProgress, queued := wb.getStats()
	assert.Equal(t, toTransfer, queued)
	assert.Equal(t, 0, inProgress)

	// now start the first maxTransfers - this should stop the timer
	for i := 0; i < maxTransfers; i++ {
		<-pis[i].started
	}

	// timer should be stopped now
	assertTimerRunning(t, wb, false)

	inProgress, queued = wb.getStats()
	assert.Equal(t, toTransfer-maxTransfers, queued)
	assert.Equal(t, maxTransfers, inProgress)

	// now finish the the first maxTransfers
	for i := 0; i < maxTransfers; i++ {
		pis[i].finish(nil)
	}

	// now start and finish the remaining transfers one at a time
	for i := maxTransfers; i < toTransfer; i++ {
		<-pis[i].started
		pis[i].finish(nil)
	}
	waitUntilNoTransfers(t, wb)

	inProgress, queued = wb.getStats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 0)
}

func TestWriteBackRemove(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	item := &Item{}

	// cancel when not in writeback
	assert.False(t, wb.remove(item))

	// add item
	pi1 := newPutItem(t)
	wbItem := wb.add(item, "one", true, pi1.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// cancel when not uploading
	assert.True(t, wb.remove(item))
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)
	assert.False(t, pi1.cancelled)

	// add item
	pi2 := newPutItem(t)
	wbItem = wb.add(item, "one", true, pi2.put)
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// wait for upload to start
	<-pi2.started

	// cancel when uploading
	assert.True(t, wb.remove(item))
	checkNotOnHeap(t, wb, wbItem)
	checkNotInLookup(t, wb, wbItem)
	assert.True(t, pi2.cancelled)
}
