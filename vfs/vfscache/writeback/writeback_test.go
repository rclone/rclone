package writeback

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"slices"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWriteBack(t *testing.T) (wb *WriteBack, cancel func()) {
	ctx, cancel := context.WithCancel(context.Background())
	opt := vfscommon.Opt
	opt.WriteBack = fs.Duration(100 * time.Millisecond)
	wb = New(ctx, &opt)
	return wb, cancel
}

// string for debugging - make a copy and pop the items out in order
func (wb *WriteBack) string(t *testing.T) string {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	ws := wb.items

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

	wb := &WriteBack{
		items: writeBackItems{},
	}

	heap.Init(&wb.items)
	assert.Equal(t, "", wb.string(t))
	heap.Push(&wb.items, &wbItem2)
	assert.Equal(t, "two", wb.string(t))
	heap.Push(&wb.items, &wbItem3)
	assert.Equal(t, "two,three", wb.string(t))
	heap.Push(&wb.items, &wbItem1)
	assert.Equal(t, "one,two,three", wb.string(t))

	wb.items._update(&wbItem1, now.Add(3*time.Second))
	assert.Equal(t, "two,one,three", wb.string(t))

	wb.items._update(&wbItem1, now.Add(5*time.Second))
	assert.Equal(t, "two,three,one", wb.string(t))

	// Set all times the same - should sort in insertion order
	wb.items._update(&wbItem1, now)
	wb.items._update(&wbItem2, now)
	wb.items._update(&wbItem3, now)
	assert.Equal(t, "one,two,three", wb.string(t))
}

func checkOnHeap(t *testing.T, wb *WriteBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.True(t, wbItem.onHeap)
	if slices.Contains(wb.items, wbItem) {
		return
	}
	assert.Failf(t, "expecting %q on heap", wbItem.name)
}

func checkNotOnHeap(t *testing.T, wb *WriteBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.False(t, wbItem.onHeap)
	for i := range wb.items {
		if wb.items[i] == wbItem {
			t.Errorf("not expecting %q on heap", wbItem.name)
		}
	}
}

func checkInLookup(t *testing.T, wb *WriteBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.Equal(t, wbItem, wb.lookup[wbItem.id])
}

func checkNotInLookup(t *testing.T, wb *WriteBack, wbItem *writeBackItem) {
	wb.mu.Lock()
	defer wb.mu.Unlock()
	assert.Nil(t, wb.lookup[wbItem.id])
}

func TestWriteBackItemCRUD(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	// _peekItem empty
	assert.Nil(t, wb._peekItem())

	wbItem1 := wb._newItem(0, "one", 10)
	checkOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)

	wbItem2 := wb._newItem(0, "two", 10)
	checkOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)

	wbItem3 := wb._newItem(0, "three", 10)
	checkOnHeap(t, wb, wbItem3)
	checkInLookup(t, wb, wbItem3)

	assert.Equal(t, "one,two,three", wb.string(t))

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
	assert.Equal(t, "two,three", wb.string(t))

	// _pushItem
	wb._pushItem(wbItem1)
	checkOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)
	assert.Equal(t, "one,two,three", wb.string(t))
	// push twice
	wb._pushItem(wbItem1)
	assert.Equal(t, "one,two,three", wb.string(t))

	// _peekItem
	assert.Equal(t, wbItem1, wb._peekItem())

	// _removeItem
	assert.Equal(t, "one,two,three", wb.string(t))
	wb._removeItem(wbItem2)
	checkNotOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.Equal(t, "one,three", wb.string(t))
	// remove twice
	wb._removeItem(wbItem2)
	checkNotOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.Equal(t, "one,three", wb.string(t))
}

func assertTimerRunning(t *testing.T, wb *WriteBack, running bool) {
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

	_ = wb._newItem(0, "three", 10)

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

// put the object as per PutFn interface
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

func waitUntilNoTransfers(t *testing.T, wb *WriteBack) {
	for range 100 {
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

	pi := newPutItem(t)

	var inID Handle
	wb.SetID(&inID)
	assert.Equal(t, Handle(1), inID)

	id := wb.Add(inID, "one", 10, true, pi.put)
	assert.Equal(t, inID, id)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.string(t))

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

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, true, pi.put)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.string(t))

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

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, true, pi.put)
	wbItem := wb.lookup[id]
	assert.Equal(t, int64(10), wbItem.size) // check size
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// Now the upload has started add another one

	pi2 := newPutItem(t)
	id2 := wb.Add(id, "one", 20, true, pi2.put)
	assert.Equal(t, id, id2)
	assert.Equal(t, int64(20), wbItem.size) // check size has changed
	checkOnHeap(t, wb, wbItem)              // object awaiting writeback time
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

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, false, pi.put)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.string(t))

	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// Now the upload has started add another one

	pi2 := newPutItem(t)
	id2 := wb.Add(id, "one", 10, false, pi2.put)
	assert.Equal(t, id, id2)
	checkNotOnHeap(t, wb, wbItem) // object still being transferred
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

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, true, pi.put)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, "one", wb.string(t))

	// Immediately add another upload before the first has started

	pi2 := newPutItem(t)
	id2 := wb.Add(id, "one", 10, true, pi2.put)
	assert.Equal(t, id, id2)
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

	pi := newPutItem(t)

	wb.Add(0, "one", 10, true, pi.put)

	inProgress, queued := wb.Stats()
	assert.Equal(t, queued, 1)
	assert.Equal(t, inProgress, 0)

	<-pi.started

	inProgress, queued = wb.Stats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 1)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)

	inProgress, queued = wb.Stats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 0)

}

func TestWriteBackQueue(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, true, pi.put)

	queue := wb.Queue()
	require.Equal(t, 1, len(queue))
	assert.Greater(t, queue[0].Expiry, 0.0)
	assert.Less(t, queue[0].Expiry, 1.0)
	queue[0].Expiry = 0.0
	assert.Equal(t, []QueueInfo{
		{
			Name:      "one",
			Size:      10,
			Expiry:    0.0,
			Tries:     0,
			Delay:     0.1,
			Uploading: false,
			ID:        id,
		},
	}, queue)

	<-pi.started

	queue = wb.Queue()
	require.Equal(t, 1, len(queue))
	assert.Less(t, queue[0].Expiry, 0.0)
	assert.Greater(t, queue[0].Expiry, -1.0)
	queue[0].Expiry = 0.0
	assert.Equal(t, []QueueInfo{
		{
			Name:      "one",
			Size:      10,
			Expiry:    0.0,
			Tries:     1,
			Delay:     0.1,
			Uploading: true,
			ID:        id,
		},
	}, queue)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)

	queue = wb.Queue()
	assert.Equal(t, []QueueInfo{}, queue)
}

func TestWriteBackSetExpiry(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	err := wb.SetExpiry(123123123, time.Now(), 0)
	assert.Equal(t, ErrorIDNotFound, err)

	pi := newPutItem(t)

	id := wb.Add(0, "one", 10, true, pi.put)
	wbItem := wb.lookup[id]

	// get the expiry time with locking so we don't cause races
	getExpiry := func() time.Time {
		wb.mu.Lock()
		defer wb.mu.Unlock()
		return wbItem.expiry
	}

	expiry := time.Until(getExpiry()).Seconds()
	assert.Greater(t, expiry, 0.0)
	assert.Less(t, expiry, 1.0)

	newExpiry := time.Now().Add(100 * time.Second)
	require.NoError(t, wb.SetExpiry(wbItem.id, newExpiry, 0))
	assert.Equal(t, newExpiry, getExpiry())

	// This starts the transfer
	newExpiry = wbItem.expiry.Add(-200 * time.Second)
	require.NoError(t, wb.SetExpiry(wbItem.id, time.Time{}, -200*time.Second))
	assert.Equal(t, newExpiry, getExpiry())

	<-pi.started

	expiry = time.Until(getExpiry()).Seconds()
	assert.LessOrEqual(t, expiry, -100.0)

	pi.finish(nil) // transfer successful
	waitUntilNoTransfers(t, wb)

	expiry = time.Until(getExpiry()).Seconds()
	assert.LessOrEqual(t, expiry, -100.0)
}

// Test queuing more than fs.Config.Transfers
func TestWriteBackMaxQueue(t *testing.T) {
	ctx := context.Background()
	ci := fs.GetConfig(ctx)
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	maxTransfers := ci.Transfers
	toTransfer := maxTransfers + 2

	// put toTransfer things in the queue
	pis := []*putItem{}
	for range toTransfer {
		pi := newPutItem(t)
		pis = append(pis, pi)
		wb.Add(0, fmt.Sprintf("number%d", 1), 10, true, pi.put)
	}

	inProgress, queued := wb.Stats()
	assert.Equal(t, toTransfer, queued)
	assert.Equal(t, 0, inProgress)

	// now start the first maxTransfers - this should stop the timer
	for i := range maxTransfers {
		<-pis[i].started
	}

	// timer should be stopped now
	assertTimerRunning(t, wb, false)

	inProgress, queued = wb.Stats()
	assert.Equal(t, toTransfer-maxTransfers, queued)
	assert.Equal(t, maxTransfers, inProgress)

	// now finish the first maxTransfers
	for i := range maxTransfers {
		pis[i].finish(nil)
	}

	// now start and finish the remaining transfers one at a time
	for i := maxTransfers; i < toTransfer; i++ {
		<-pis[i].started
		pis[i].finish(nil)
	}
	waitUntilNoTransfers(t, wb)

	inProgress, queued = wb.Stats()
	assert.Equal(t, queued, 0)
	assert.Equal(t, inProgress, 0)
}

func TestWriteBackRename(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	// cancel when not in writeback
	wb.Rename(1, "nonExistent")

	// add item
	pi1 := newPutItem(t)
	id := wb.Add(0, "one", 10, true, pi1.put)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, wbItem.name, "one")

	// rename when not uploading
	wb.Rename(id, "two")
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.False(t, pi1.cancelled)
	assert.Equal(t, wbItem.name, "two")

	// add item
	pi2 := newPutItem(t)
	id = wb.Add(id, "two", 10, true, pi2.put)
	wbItem = wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.Equal(t, wbItem.name, "two")

	// wait for upload to start
	<-pi2.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// rename when uploading - goes back on heap
	wb.Rename(id, "three")
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.True(t, pi2.cancelled)
	assert.Equal(t, wbItem.name, "three")
}

// TestWriteBackRenameDuplicates checks that if we rename an entry and
// make a duplicate, we remove the duplicate.
func TestWriteBackRenameDuplicates(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	// add item "one", 10
	pi1 := newPutItem(t)
	id1 := wb.Add(0, "one", 10, true, pi1.put)
	wbItem1 := wb.lookup[id1]
	checkOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)
	assert.Equal(t, wbItem1.name, "one")

	<-pi1.started
	checkNotOnHeap(t, wb, wbItem1)
	checkInLookup(t, wb, wbItem1)

	// add item "two"
	pi2 := newPutItem(t)
	id2 := wb.Add(0, "two", 10, true, pi2.put)
	wbItem2 := wb.lookup[id2]
	checkOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.Equal(t, wbItem2.name, "two")

	<-pi2.started
	checkNotOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)

	// rename "two" to "one"
	wb.Rename(id2, "one")

	// check "one" is cancelled and removed from heap and lookup
	checkNotOnHeap(t, wb, wbItem1)
	checkNotInLookup(t, wb, wbItem1)
	assert.True(t, pi1.cancelled)
	assert.Equal(t, wbItem1.name, "one")

	// check "two" (now called "one"!) has been cancelled and will
	// be retried
	checkOnHeap(t, wb, wbItem2)
	checkInLookup(t, wb, wbItem2)
	assert.True(t, pi2.cancelled)
	assert.Equal(t, wbItem2.name, "one")
}

func TestWriteBackCancelUpload(t *testing.T) {
	wb, cancel := newTestWriteBack(t)
	defer cancel()

	// cancel when not in writeback
	assert.False(t, wb.cancelUpload(1))

	// add item
	pi := newPutItem(t)
	id := wb.Add(0, "one", 10, true, pi.put)
	wbItem := wb.lookup[id]
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// cancel when not uploading
	assert.False(t, wb.cancelUpload(id))
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// wait for upload to start
	<-pi.started
	checkNotOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)

	// cancel when uploading
	assert.True(t, wb.cancelUpload(id))
	checkOnHeap(t, wb, wbItem)
	checkInLookup(t, wb, wbItem)
	assert.True(t, pi.cancelled)
}
