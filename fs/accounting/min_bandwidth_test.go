package accounting

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/require"
)

// averageLoop evaluates the stall check once per second, so every timing
// below is expressed in whole ticks. A window of 2s means a cancellation
// cannot land before the third tick: one to arm belowMinSince, then a full
// window on top of it.
const (
	testMinBandwidth     = 1000            // bytes/sec required
	testMinBandwidthTime = 2 * time.Second // ...sustained this long
	testTrickleInterval  = 200 * time.Millisecond
	testTrickleBytes     = 2       // 2 bytes/200ms = 10 bytes/sec, well under testMinBandwidth
	testBurstChunk       = 1 << 16 // 64KiB/200ms ~= 327KiB/s, comfortably over it
)

// minBandwidthContext returns a context carrying a PRIVATE config with the
// min-bandwidth knobs set, plus the Account built from it.
//
// The config must be private: ci.MinBandwidth and ci.MinBandwidthTime are
// read every second from averageLoop's goroutine for every live Account in
// the process, including ones belonging to other tests in this package. A
// test that set them on the shared global config would race those reads and
// cancel unrelated transfers.
func minBandwidthContext(t *testing.T, name string, minBandwidth fs.SizeSuffix) (*Account, context.Context) {
	t.Helper()
	ctx, ci := fs.AddConfig(context.Background())
	ci.MinBandwidth = minBandwidth
	ci.MinBandwidthTime = fs.Duration(testMinBandwidthTime)

	in := io.NopCloser(bytes.NewBuffer(make([]byte, 100)))
	acc := newAccountSizeName(ctx, NewStats(ctx), in, -1, name)
	t.Cleanup(acc.Done)
	return acc, ctx
}

// TestAccountMinBandwidthDisabledByDefault proves --min-bandwidth's default
// (0, disabled) is a genuine no-op: a transfer moving well under any
// reasonable rate is never cancelled when the check isn't opted into.
//
// The trickle runs longer than testMinBandwidthTime plus the tick needed to
// arm it, so this is the same shape of transfer that
// TestAccountMinBandwidthDetectsStall cancels -- the only difference is that
// MinBandwidth is left at 0. Flipping that default, or making the check fire
// when it is unset, therefore fails here.
func TestAccountMinBandwidthDisabledByDefault(t *testing.T) {
	require.Equal(t, fs.SizeSuffix(0), fs.GetConfig(context.Background()).MinBandwidth,
		"test assumes default config -- MinBandwidth should be 0/unset")

	acc, _ := minBandwidthContext(t, "test-disabled", 0)

	deadline := time.Now().Add(testMinBandwidthTime + 2*time.Second)
	for time.Now().Before(deadline) {
		require.NoError(t, acc.AccountRead(testTrickleBytes))
		time.Sleep(testTrickleInterval)
	}

	require.NoError(t, context.Cause(acc.Context()),
		"Context() was cancelled despite --min-bandwidth being unset (0)")
}

// TestAccountMinBandwidthDetectsStall is the fix side of the fshttp repro in
// issue #9841. That repro showed a connection trickling data slower than a
// real transfer, but faster than --timeout, can hold a request open
// indefinitely with no error. This drives the exact same shape of trickle
// through the real accounting path (AccountRead, the same call
// rw.SetAccounting wires up for multipart chunk uploads -- see
// lib/multipart.UploadMultipart) and proves the transfer's own Context()
// gets cancelled once it has been below --min-bandwidth for
// --min-bandwidth-time, with a cause any caller can inspect via
// context.Cause.
func TestAccountMinBandwidthDetectsStall(t *testing.T) {
	acc, _ := minBandwidthContext(t, "test-stalled", testMinBandwidth)

	stop := make(chan struct{})
	defer close(stop)
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(testTrickleInterval):
				// Errors expected once the stall is detected -- this
				// goroutine's job is just to keep trickling, not to assert.
				_ = acc.AccountRead(testTrickleBytes)
			}
		}
	}()

	start := time.Now()
	require.Eventually(t, func() bool {
		return context.Cause(acc.Context()) != nil
	}, testMinBandwidthTime+3*time.Second, 50*time.Millisecond,
		"stall was not detected within minBandwidthTime + slack")
	elapsed := time.Since(start)

	cause := context.Cause(acc.Context())
	require.ErrorIs(t, cause, ErrorTransferStalled)
	require.GreaterOrEqual(t, elapsed, testMinBandwidthTime,
		"detected before a full --min-bandwidth-time window even completed")

	// Whatever is driving this transfer's real Read/AccountRead calls must
	// see the same failure -- this is what actually stops the transfer and
	// feeds --retries, not the context cancellation alone.
	require.ErrorIs(t, acc.AccountRead(1), ErrorTransferStalled)

	// The stall is raised once, not once per tick: on the paths where the
	// cancellation cannot take effect the transfer keeps running, and an
	// unlatched check would log for its entire remaining lifetime.
	acc.values.mu.Lock()
	fired := acc.values.stallFired
	acc.values.mu.Unlock()
	require.True(t, fired, "stall should be latched after firing")
	acc.values.mu.Lock()
	again, _ := acc.stallCheckLocked(time.Now().Add(time.Hour))
	acc.values.mu.Unlock()
	require.False(t, again, "stall re-fired on a later tick -- it must be raised only once")

	t.Logf("--min-bandwidth=%d/s --min-bandwidth-time=%v; stall against a ~10 bytes/sec trickle "+
		"detected after %v with: %v", testMinBandwidth, testMinBandwidthTime, elapsed, cause)
}

// TestAccountMinBandwidthStallIsRetryable proves the stall error is marked
// retryable, so --retries re-drives the transfer deliberately rather than
// incidentally: a stall is the case most likely to succeed on a fresh
// attempt. StatsInfo.Error routes on IsRetryError, so an unmarked error
// would be counted as a plain failure.
//
// Note this covers --retries, NOT --low-level-retries: that loop runs inside
// the backend on fserrors.ShouldRetry, which consults Timeout()/Temporary()
// rather than Retry(), and in any case the backend sees the transport's
// context.Canceled, never this error.
func TestAccountMinBandwidthStallIsRetryable(t *testing.T) {
	require.True(t, fserrors.IsRetryError(ErrorTransferStalled),
		"ErrorTransferStalled must be marked retryable or --retries counts a stall as a hard failure")
	require.False(t, fserrors.IsNoRetryError(ErrorTransferStalled))

	// The wrapping stallCheckLocked applies must preserve both properties.
	wrapped := fmt.Errorf("%w: 9/s averaged over the last 2s (want >= 1000/s)", ErrorTransferStalled)
	require.True(t, fserrors.IsRetryError(wrapped))
	require.ErrorIs(t, wrapped, ErrorTransferStalled)
}

// TestStallCheckResetsWindowOnRecovery drives stallCheckLocked directly, at
// chosen instants, because the reset it covers cannot be reached in
// reasonable wall-clock time through AccountRead: avg is an EWMA over 16
// samples, so falling back below the threshold after a burst takes ~33
// one-second ticks.
//
// What the reset buys is that a dip which recovers does not leave a stale
// belowMinSince behind, so a LATER dip gets a full --min-bandwidth-time
// window of its own rather than inheriting the first dip's head start.
// Without it, the second dip here cancels immediately.
func TestStallCheckResetsWindowOnRecovery(t *testing.T) {
	acc, _ := minBandwidthContext(t, "test-reset", testMinBandwidth)

	// Held throughout: averageLoop calls stallCheckLocked under this same
	// lock, and would otherwise overwrite avg between steps.
	acc.values.mu.Lock()
	defer acc.values.mu.Unlock()
	t0 := time.Now()

	acc.values.avg = 10 // below the minimum: arms the window
	cancel, _ := acc.stallCheckLocked(t0)
	require.False(t, cancel)
	require.Equal(t, t0, acc.values.belowMinSince)

	acc.values.avg = 2 * testMinBandwidth // recovered: must clear the window
	cancel, _ = acc.stallCheckLocked(t0.Add(time.Second))
	require.False(t, cancel)
	require.True(t, acc.values.belowMinSince.IsZero(),
		"recovering above --min-bandwidth must reset the stall window")

	acc.values.avg = 10 // dips again, a full --min-bandwidth-time after t0
	cancel, _ = acc.stallCheckLocked(t0.Add(testMinBandwidthTime))
	require.False(t, cancel,
		"a fresh dip must start a new window, not inherit the first dip's elapsed time")

	// ...and only then, a full window after the SECOND dip began.
	cancel, cause := acc.stallCheckLocked(t0.Add(2 * testMinBandwidthTime))
	require.True(t, cancel)
	require.ErrorIs(t, cause, ErrorTransferStalled)
}

// TestAccountMinBandwidthRecovers proves a transfer that dips below
// --min-bandwidth briefly, then recovers before --min-bandwidth-time
// elapses, is NOT cancelled -- only a SUSTAINED shortfall should be. This
// matters because legitimate short stalls are common: a bursty network is
// routine, and a server-side operation can pause mid-transfer.
//
// stallCheckLocked resets belowMinSince the moment avg clears the threshold
// again, and that reset is what is under test: the run must outlive
// belowMinSince + --min-bandwidth-time, or the assertion holds for the
// trivial reason that no tick could have cancelled yet. Deleting the reset
// branch must make this fail, so the burst phase runs past the third tick.
func TestAccountMinBandwidthRecovers(t *testing.T) {
	acc, _ := minBandwidthContext(t, "test-recovers", testMinBandwidth)

	// Under the minimum for ~1.5s: the tick at ~1s arms belowMinSince.
	dipUntil := time.Now().Add(1500 * time.Millisecond)
	for time.Now().Before(dipUntil) {
		require.NoError(t, acc.AccountRead(testTrickleBytes))
		time.Sleep(testTrickleInterval)
	}

	// Then comfortably over it, past belowMinSince + --min-bandwidth-time
	// (~3s). Without the reset, the tick at ~3s cancels.
	burstUntil := time.Now().Add(2500 * time.Millisecond)
	for time.Now().Before(burstUntil) {
		require.NoError(t, acc.AccountRead(testBurstChunk))
		time.Sleep(testTrickleInterval)
	}

	require.NoError(t, context.Cause(acc.Context()),
		"cancelled despite recovering above --min-bandwidth before --min-bandwidth-time elapsed")
}
