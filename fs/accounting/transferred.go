package accounting

import (
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

// transferred holds a synchronized map of in progress transfers.
type transferred struct {
	done          chan struct{}
	checkDuration time.Duration
	interval      time.Duration
	mu            sync.Mutex
	items         []AccountSnapshot
}

// newTransferred makes a new transferred object.
func newTransferred() *transferred {
	tr := &transferred{
		items:         []AccountSnapshot{},
		checkDuration: fs.Config.TransferredExpireDuration,
		interval:      fs.Config.TransferredExpireInterval,
		done:          make(chan struct{}),
	}
	go tr.watchForExpired()
	return tr
}

// add adds new snapshot to the transferred list.
func (tr *transferred) add(acc AccountSnapshot) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	tr.items = append(tr.items, acc)
}

// watchForExpired removes all snaphots older than maxAge.
func (tr *transferred) watchForExpired() {
	ticker := time.NewTicker(tr.interval)
	for {
		select {
		case <-tr.done:
			return
		case <-ticker.C:
			tr.clearExpired(tr.checkDuration)
		}
	}
}

// clearExpired removes all snaphots older than maxAge.
func (tr *transferred) clearExpired(maxAge time.Duration) {
	limit := time.Now().UTC().Add(-maxAge)
	tr.mu.Lock()
	defer tr.mu.Unlock()
	// In place filtering by reusing items storage.
	tmp := tr.items[:0]
	for _, i := range tr.items {
		if limit.Before(time.Time(i.Timestamp)) {
			tmp = append(tmp, i)
		}
	}
	tr.items = tmp
}

func (tr *transferred) snapshots() []AccountSnapshot {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.items
}

func (tr *transferred) close() {
	close(tr.done)
}
