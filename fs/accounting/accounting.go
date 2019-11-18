// Package accounting providers an accounting and limiting reader
package accounting

import (
	"fmt"
	"io"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/rclone/rclone/fs/rc"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/fserrors"
)

// ErrorMaxTransferLimitReached is returned from Read when the max
// transfer limit is reached.
var ErrorMaxTransferLimitReached = fserrors.FatalError(errors.New("Max transfer limit reached as set by --max-transfer"))

// Account limits and accounts for one transfer
type Account struct {
	stats *StatsInfo
	// The mutex is to make sure Read() and Close() aren't called
	// concurrently.  Unfortunately the persistent connection loop
	// in http transport calls Read() after Do() returns on
	// CancelRequest so this race can happen when it apparently
	// shouldn't.
	mu      sync.Mutex
	in      io.Reader
	origIn  io.ReadCloser
	close   io.Closer
	size    int64
	name    string
	statmu  sync.Mutex    // Separate mutex for stat values.
	bytes   int64         // Total number of bytes read
	max     int64         // if >=0 the max number of bytes to transfer
	start   time.Time     // Start time of first read
	lpTime  time.Time     // Time of last average measurement
	lpBytes int           // Number of bytes read since last measurement
	avg     float64       // Moving average of last few measurements in bytes/s
	closed  bool          // set if the file is closed
	exit    chan struct{} // channel that will be closed when transfer is finished
	withBuf bool          // is using a buffered in
}

const averagePeriod = 16 // period to do exponentially weighted averages over

// newAccountSizeName makes a Account reader for an io.ReadCloser of
// the given size and name
func newAccountSizeName(stats *StatsInfo, in io.ReadCloser, size int64, name string) *Account {
	acc := &Account{
		stats:  stats,
		in:     in,
		close:  in,
		origIn: in,
		size:   size,
		name:   name,
		exit:   make(chan struct{}),
		avg:    0,
		lpTime: time.Now(),
		max:    int64(fs.Config.MaxTransfer),
	}
	go acc.averageLoop()
	stats.inProgress.set(acc.name, acc)
	return acc
}

// WithBuffer - If the file is above a certain size it adds an Async reader
func (acc *Account) WithBuffer() *Account {
	// if already have a buffer then just return
	if acc.withBuf {
		return acc
	}
	acc.withBuf = true
	var buffers int
	if acc.size >= int64(fs.Config.BufferSize) || acc.size == -1 {
		buffers = int(int64(fs.Config.BufferSize) / asyncreader.BufferSize)
	} else {
		buffers = int(acc.size / asyncreader.BufferSize)
	}
	// On big files add a buffer
	if buffers > 0 {
		rc, err := asyncreader.New(acc.origIn, buffers)
		if err != nil {
			fs.Errorf(acc.name, "Failed to make buffer: %v", err)
		} else {
			acc.in = rc
			acc.close = rc
		}
	}
	return acc
}

// GetReader returns the underlying io.ReadCloser under any Buffer
func (acc *Account) GetReader() io.ReadCloser {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.origIn
}

// GetAsyncReader returns the current AsyncReader or nil if Account is unbuffered
func (acc *Account) GetAsyncReader() *asyncreader.AsyncReader {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if asyncIn, ok := acc.in.(*asyncreader.AsyncReader); ok {
		return asyncIn
	}
	return nil
}

// StopBuffering stops the async buffer doing any more buffering
func (acc *Account) StopBuffering() {
	if asyncIn, ok := acc.in.(*asyncreader.AsyncReader); ok {
		asyncIn.Abandon()
	}
}

// UpdateReader updates the underlying io.ReadCloser stopping the
// async buffer (if any) and re-adding it
func (acc *Account) UpdateReader(in io.ReadCloser) {
	acc.mu.Lock()
	withBuf := acc.withBuf
	if withBuf {
		acc.StopBuffering()
		acc.withBuf = false
	}
	acc.in = in
	acc.close = in
	acc.origIn = in
	acc.closed = false
	if withBuf {
		acc.WithBuffer()
	}
	acc.mu.Unlock()
}

// averageLoop calculates averages for the stats in the background
func (acc *Account) averageLoop() {
	tick := time.NewTicker(time.Second)
	var period float64
	defer tick.Stop()
	for {
		select {
		case now := <-tick.C:
			acc.statmu.Lock()
			// Add average of last second.
			elapsed := now.Sub(acc.lpTime).Seconds()
			avg := float64(acc.lpBytes) / elapsed
			// Soft start the moving average
			if period < averagePeriod {
				period++
			}
			acc.avg = (avg + (period-1)*acc.avg) / period
			acc.lpBytes = 0
			acc.lpTime = now
			// Unlock stats
			acc.statmu.Unlock()
		case <-acc.exit:
			return
		}
	}
}

// Check the read is valid
func (acc *Account) checkRead() (err error) {
	acc.statmu.Lock()
	if acc.max >= 0 && acc.stats.GetBytes() >= acc.max {
		acc.statmu.Unlock()
		return ErrorMaxTransferLimitReached
	}
	// Set start time.
	if acc.start.IsZero() {
		acc.start = time.Now()
	}
	acc.statmu.Unlock()
	return nil
}

// ServerSideCopyStart should be called at the start of a server side copy
//
// This pretends a transfer has started
func (acc *Account) ServerSideCopyStart() {
	acc.statmu.Lock()
	// Set start time.
	if acc.start.IsZero() {
		acc.start = time.Now()
	}
	acc.statmu.Unlock()
}

// ServerSideCopyEnd accounts for a read of n bytes in a sever side copy
func (acc *Account) ServerSideCopyEnd(n int64) {
	// Update Stats
	acc.statmu.Lock()
	acc.bytes += n
	acc.statmu.Unlock()

	acc.stats.Bytes(n)
}

// Account the read and limit bandwidth
func (acc *Account) accountRead(n int) {
	// Update Stats
	acc.statmu.Lock()
	acc.lpBytes += n
	acc.bytes += int64(n)
	acc.statmu.Unlock()

	acc.stats.Bytes(int64(n))

	limitBandwidth(n)
}

// read bytes from the io.Reader passed in and account them
func (acc *Account) read(in io.Reader, p []byte) (n int, err error) {
	err = acc.checkRead()
	if err == nil {
		n, err = in.Read(p)
		acc.accountRead(n)
	}
	return n, err
}

// Read bytes from the object - see io.Reader
func (acc *Account) Read(p []byte) (n int, err error) {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.read(acc.in, p)
}

// AccountRead account having read n bytes
func (acc *Account) AccountRead(n int) (err error) {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	err = acc.checkRead()
	if err == nil {
		acc.accountRead(n)
	}
	return err
}

// Close the object
func (acc *Account) Close() error {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	if acc.closed {
		return nil
	}
	acc.closed = true
	if acc.close == nil {
		return nil
	}
	return acc.close.Close()
}

// Done with accounting - must be called to free accounting goroutine
func (acc *Account) Done() {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	close(acc.exit)
	acc.stats.inProgress.clear(acc.name)
}

// progress returns bytes read as well as the size.
// Size can be <= 0 if the size is unknown.
func (acc *Account) progress() (bytes, size int64) {
	if acc == nil {
		return 0, 0
	}
	acc.statmu.Lock()
	bytes, size = acc.bytes, acc.size
	acc.statmu.Unlock()
	return bytes, size
}

// speed returns the speed of the current file transfer
// in bytes per second, as well a an exponentially weighted moving average
// If no read has completed yet, 0 is returned for both values.
func (acc *Account) speed() (bps, current float64) {
	if acc == nil {
		return 0, 0
	}
	acc.statmu.Lock()
	defer acc.statmu.Unlock()
	if acc.bytes == 0 {
		return 0, 0
	}
	// Calculate speed from first read.
	total := float64(time.Now().Sub(acc.start)) / float64(time.Second)
	bps = float64(acc.bytes) / total
	current = acc.avg
	return
}

// eta returns the ETA of the current operation,
// rounded to full seconds.
// If the ETA cannot be determined 'ok' returns false.
func (acc *Account) eta() (etaDuration time.Duration, ok bool) {
	if acc == nil {
		return 0, false
	}
	acc.statmu.Lock()
	defer acc.statmu.Unlock()
	return eta(acc.bytes, acc.size, acc.avg)
}

// shortenName shortens in to size runes long
// If size <= 0 then in is left untouched
func shortenName(in string, size int) string {
	if size <= 0 {
		return in
	}
	if utf8.RuneCountInString(in) <= size {
		return in
	}
	name := []rune(in)
	size-- // don't count elipsis rune
	suffixLength := size / 2
	prefixLength := size - suffixLength
	suffixStart := len(name) - suffixLength
	name = append(append(name[:prefixLength], '…'), name[suffixStart:]...)
	return string(name)
}

// String produces stats for this file
func (acc *Account) String() string {
	a, b := acc.progress()
	_, cur := acc.speed()
	eta, etaok := acc.eta()
	etas := "-"
	if etaok {
		if eta > 0 {
			etas = fmt.Sprintf("%v", eta)
		} else {
			etas = "0s"
		}
	}

	if fs.Config.DataRateUnit == "bits" {
		cur = cur * 8
	}

	percentageDone := 0
	if b > 0 {
		percentageDone = int(100 * float64(a) / float64(b))
	}

	return fmt.Sprintf("%*s:%3d%% /%s, %s/s, %s",
		fs.Config.StatsFileNameLength,
		shortenName(acc.name, fs.Config.StatsFileNameLength),
		percentageDone,
		fs.SizeSuffix(b),
		fs.SizeSuffix(cur),
		etas,
	)
}

// RemoteStats produces stats for this file
func (acc *Account) RemoteStats() (out rc.Params) {
	out = make(rc.Params)
	a, b := acc.progress()
	out["bytes"] = a
	out["size"] = b
	spd, cur := acc.speed()
	out["speed"] = spd
	out["speedAvg"] = cur

	eta, etaok := acc.eta()
	out["eta"] = nil
	if etaok {
		if eta > 0 {
			out["eta"] = eta.Seconds()
		} else {
			out["eta"] = 0
		}
	}
	out["name"] = acc.name

	percentageDone := 0
	if b > 0 {
		percentageDone = int(100 * float64(a) / float64(b))
	}
	out["percentage"] = percentageDone

	return out
}

// OldStream returns the top io.Reader
func (acc *Account) OldStream() io.Reader {
	acc.mu.Lock()
	defer acc.mu.Unlock()
	return acc.in
}

// SetStream updates the top io.Reader
func (acc *Account) SetStream(in io.Reader) {
	acc.mu.Lock()
	acc.in = in
	acc.mu.Unlock()
}

// WrapStream wraps an io Reader so it will be accounted in the same
// way as account
func (acc *Account) WrapStream(in io.Reader) io.Reader {
	return &accountStream{
		acc: acc,
		in:  in,
	}
}

// accountStream accounts a single io.Reader into a parent *Account
type accountStream struct {
	acc *Account
	in  io.Reader
}

// OldStream return the underlying stream
func (a *accountStream) OldStream() io.Reader {
	return a.in
}

// SetStream set the underlying stream
func (a *accountStream) SetStream(in io.Reader) {
	a.in = in
}

// WrapStream wrap in in an accounter
func (a *accountStream) WrapStream(in io.Reader) io.Reader {
	return a.acc.WrapStream(in)
}

// Read bytes from the object - see io.Reader
func (a *accountStream) Read(p []byte) (n int, err error) {
	return a.acc.read(a.in, p)
}

// Accounter accounts a stream allowing the accounting to be removed and re-added
type Accounter interface {
	io.Reader
	OldStream() io.Reader
	SetStream(io.Reader)
	WrapStream(io.Reader) io.Reader
}

// WrapFn wraps an io.Reader (for accounting purposes usually)
type WrapFn func(io.Reader) io.Reader

// UnWrap unwraps a reader returning unwrapped and wrap, a function to
// wrap it back up again.  If `in` is an Accounter then this function
// will take the accounting unwrapped and wrap will put it back on
// again the new Reader passed in.
//
// This allows functions which wrap io.Readers to move the accounting
// to the end of the wrapped chain of readers.  This is very important
// if buffering is being introduced and if the Reader might be wrapped
// again.
func UnWrap(in io.Reader) (unwrapped io.Reader, wrap WrapFn) {
	acc, ok := in.(Accounter)
	if !ok {
		return in, func(r io.Reader) io.Reader { return r }
	}
	return acc.OldStream(), acc.WrapStream
}
