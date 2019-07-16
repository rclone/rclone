package accounting

import (
	"io"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
)

// Transfer keeps track of initiated transfers and provides access to
// accounting functions.
// Transfer needs to be closed on completion.
type Transfer struct {
	stats  *StatsInfo
	acc    *Account
	remote string
	size   int64

	mu          sync.Mutex
	startedAt   time.Time
	completedAt time.Time
}

// newTransfer instantiates new transfer
func newTransfer(stats *StatsInfo, obj fs.Object) *Transfer {
	return newTransferRemoteSize(stats, obj.Remote(), obj.Size())
}

func newTransferRemoteSize(stats *StatsInfo, remote string, size int64) *Transfer {
	tr := &Transfer{
		stats:     stats,
		remote:    remote,
		size:      size,
		startedAt: time.Now(),
	}
	stats.AddTransfer(tr)
	return tr
}

// Done ends the transfer.
// Must be called after transfer is finished to run proper cleanups.
func (tr *Transfer) Done(err error) {
	if err != nil {
		tr.stats.Error(err)
	}
	if tr.acc != nil {
		if err := tr.acc.Close(); err != nil {
			fs.LogLevelPrintf(fs.Config.StatsLogLevel, nil, "can't close account: %+v\n", err)
		}
	}
	tr.stats.DoneTransferring(tr.remote, err == nil)
	tr.mu.Lock()
	tr.completedAt = time.Now()
	tr.mu.Unlock()
}

// Account returns reader that knows how to keep track of transfer progress.
func (tr *Transfer) Account(in io.ReadCloser) *Account {
	if tr.acc != nil {
		return tr.acc
	}
	return newAccountSizeName(tr.stats, in, tr.size, tr.remote)
}

// TimeRange returns the time transfer started and ended at. If not completed
// it will return zero time for end time.
func (tr *Transfer) TimeRange() (time.Time, time.Time) {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.startedAt, tr.completedAt
}
