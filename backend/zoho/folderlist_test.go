package zoho

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/stretchr/testify/assert"
)

// maxRolling returns the largest number of grants falling inside any
// half-open (t-window, t] interval - the quantity Zoho's throttle caps.
func maxRolling(grants []time.Time, window time.Duration) int {
	mx := 0
	for i := range grants {
		c := 0
		for j := 0; j <= i; j++ {
			if grants[i].Sub(grants[j]) < window {
				c++
			}
		}
		mx = max(mx, c)
	}
	return mx
}

func TestFolderListLimiter(t *testing.T) {
	// Isolate from the process-wide registry and restore it afterwards, so this
	// test neither observes nor leaves shared state for integration runs in the
	// same process.
	saved := folderListLimiterRegistry
	folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}
	defer func() { folderListLimiterRegistry = saved }()

	window := time.Duration(defaultListFolderWindow)
	newFs := func(region string) *Fs {
		f := &Fs{}
		f.opt.Region = region
		f.opt.ListFolderWindow = defaultListFolderWindow
		f.opt.ListFolderLimit = defaultListFolderLimit
		f.opt.ListFolderBurst = defaultListFolderBurst
		return f
	}
	f := newFs("eu")

	// The defaults must satisfy the limiter's own precondition (burst is carved
	// out of the limit, so it has to leave at least one paced token).
	assert.Less(t, defaultListFolderBurst, defaultListFolderLimit, "default burst must be below default limit")

	// Same region+folder shares one limiter; a different folder, or the same
	// folder id in a different region, is keyed separately.
	limA := f.folderListLimiter("A")
	assert.Same(t, limA, f.folderListLimiter("A"), "same folder shares one limiter")
	assert.NotSame(t, limA, f.folderListLimiter("B"), "different folder gets its own limiter")
	assert.NotSame(t, limA, newFs("com").folderListLimiter("A"), "same folder id in another region is distinct")

	// Idle eviction: an entry not listed within the window is dropped on the
	// next sweep. Age folder A and the last sweep by hand (no sleeping), then a
	// listing of a different folder triggers the sweep.
	reg := folderListLimiterRegistry
	keyA := f.opt.Region + "\x00" + "A"
	reg.mu.Lock()
	reg.limiters[keyA].lastListed = time.Now().Add(-2 * window)
	reg.lastSweep = time.Now().Add(-2 * window)
	reg.mu.Unlock()

	f.folderListLimiter("C") // triggers the sweep

	reg.mu.Lock()
	_, aKept := reg.limiters[keyA]
	_, cKept := reg.limiters[f.opt.Region+"\x00"+"C"]
	reg.mu.Unlock()
	assert.False(t, aKept, "stale folder A is evicted")
	assert.True(t, cKept, "freshly listed folder C is kept")
}

// TestFolderWindowLimiterShape drives reserve with a synthetic clock and
// checks the promised flow: a burst at the window start, the rest of the
// budget paced across the window, and the burst RE-ARMING at the next window
// boundary (delayed only by the sliding safety margin).
func TestFolderWindowLimiterShape(t *testing.T) {
	saved := folderListLimiterRegistry
	folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}
	defer func() { folderListLimiterRegistry = saved }()

	f := &Fs{}
	f.opt.Region = "eu"
	f.opt.ListFolderWindow = defaultListFolderWindow
	f.opt.ListFolderLimit = defaultListFolderLimit
	f.opt.ListFolderBurst = defaultListFolderBurst
	lim := f.folderListLimiter("shape")

	window := time.Duration(defaultListFolderWindow)
	interval := window / time.Duration(defaultListFolderLimit-defaultListFolderBurst)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var grants []time.Time
	reserve := func(now time.Time) time.Time {
		g := lim.reserve(now)
		grants = append(grants, g)
		return g
	}

	// Window 1: the first burst grants pass immediately...
	for i := 0; i < defaultListFolderBurst; i++ {
		assert.Equal(t, base, reserve(base), "burst grant %d passes immediately", i+1)
	}
	// ...then the paced phase spends the rest of the budget one interval apart
	// (greedy demand: the caller comes back as soon as the last grant is due).
	g := base
	for k := 1; k <= defaultListFolderLimit-defaultListFolderBurst; k++ {
		g = reserve(g)
		assert.Equal(t, base.Add(time.Duration(k)*interval), g, "paced grant %d", k)
	}
	assert.Less(t, g.Sub(base), window, "the whole budget fits inside the window")

	// Budget exhausted: the next grant jumps to the window boundary, where the
	// burst re-arms; the sliding safety log defers it by its margin so the
	// oldest grant has left Zoho's window.
	reburst := reserve(g)
	assert.Equal(t, base.Add(window+folderListSafetyMargin), reburst, "re-burst at the next window boundary")
	for i := 1; i < defaultListFolderBurst; i++ {
		assert.Equal(t, reburst, reserve(reburst), "window 2 burst grant %d is immediate", i+1)
	}
	// And window 2's paced phase continues from there.
	assert.Equal(t, reburst.Add(interval), reserve(reburst), "window 2 paced phase")

	assert.LessOrEqual(t, maxRolling(grants, window), defaultListFolderLimit, "never more than limit grants in any rolling window")
}

// TestFolderWindowLimiterIdleResume checks the corner the sliding safety log
// exists for: consuming a full budget, going idle into the middle of a later
// window and resuming - the rolling cap must hold across the grid boundaries.
func TestFolderWindowLimiterIdleResume(t *testing.T) {
	saved := folderListLimiterRegistry
	folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}
	defer func() { folderListLimiterRegistry = saved }()

	f := &Fs{}
	f.opt.Region = "eu"
	f.opt.ListFolderWindow = defaultListFolderWindow
	f.opt.ListFolderLimit = defaultListFolderLimit
	f.opt.ListFolderBurst = defaultListFolderBurst
	lim := f.folderListLimiter("resume")

	window := time.Duration(defaultListFolderWindow)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	var grants []time.Time
	g := base
	// Spend window 1's full budget greedily.
	for i := 0; i < defaultListFolderLimit; i++ {
		g = lim.reserve(g)
		grants = append(grants, g)
	}
	// Resume mid-window-2 after an idle gap and hammer across the 2->3
	// boundary: a fresh burst fires on resume and again at the boundary.
	g = base.Add(window + window/2)
	for i := 0; i < defaultListFolderLimit; i++ {
		g = lim.reserve(g)
		grants = append(grants, g)
	}
	assert.LessOrEqual(t, maxRolling(grants, window), defaultListFolderLimit, "rolling cap holds across idle resume and grid boundaries")
}

// TestFolderWindowLimiterClamps checks the burst/limit clamps behaviourally.
func TestFolderWindowLimiterClamps(t *testing.T) {
	saved := folderListLimiterRegistry
	folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}
	defer func() { folderListLimiterRegistry = saved }()

	window := time.Duration(defaultListFolderWindow)
	base := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	newFs := func(limit, burst int) *Fs {
		f := &Fs{}
		f.opt.Region = "eu"
		f.opt.ListFolderWindow = defaultListFolderWindow
		f.opt.ListFolderLimit = limit
		f.opt.ListFolderBurst = burst
		return f
	}

	// burst 0 clamps to 1: pacing starts from the second listing.
	lim := newFs(defaultListFolderLimit, 0).folderListLimiter("Z")
	assert.Equal(t, base, lim.reserve(base))
	assert.Equal(t, base.Add(window/time.Duration(defaultListFolderLimit-1)), lim.reserve(base), "second listing is paced")

	// burst >= limit clamps to limit-1, leaving one paced token per window.
	lim = newFs(5, 99).folderListLimiter("Y")
	for i := 0; i < 4; i++ {
		assert.Equal(t, base, lim.reserve(base), "clamped burst grant %d", i+1)
	}
	assert.Equal(t, base.Add(window), lim.reserve(base), "5th grant rolls into the next window")

	// limit 1 keeps a single burst token: one listing per window (+margin once
	// the safety log is full).
	lim = newFs(1, 3).folderListLimiter("X")
	assert.Equal(t, base, lim.reserve(base))
	assert.Equal(t, base.Add(window+folderListSafetyMargin), lim.reserve(base), "limit 1 waits a whole window")
}

// TestFolderListLimiterConcurrent hammers the process-wide, mutex-guarded
// registry and one shared limiter from many goroutines at once. Correctness is
// proven by `go test -race`; the rolling-cap invariant is asserted after the
// fact. reserve never sleeps, so this still runs in ~0s.
func TestFolderListLimiterConcurrent(t *testing.T) {
	saved := folderListLimiterRegistry
	folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}
	defer func() { folderListLimiterRegistry = saved }()

	f := &Fs{}
	f.opt.Region = "eu"
	f.opt.ListFolderWindow = fs.Duration(time.Hour) // large so nothing is evicted mid-test
	f.opt.ListFolderLimit = 19
	f.opt.ListFolderBurst = 6

	const workers, folders = 64, 5
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			f.folderListLimiter(fmt.Sprintf("dir-%d", i%folders)).reserve(time.Now())
		}(i)
	}
	wg.Wait()

	// Exactly one shared limiter per distinct folder, whatever the interleaving.
	assert.Len(t, folderListLimiterRegistry.limiters, folders)
	// And each limiter granted every reservation without breaching its own cap:
	// grants are capped at limit entries and non-decreasing.
	for _, e := range folderListLimiterRegistry.limiters {
		assert.LessOrEqual(t, len(e.limiter.grants), e.limiter.limit)
		for i := 1; i < len(e.limiter.grants); i++ {
			assert.False(t, e.limiter.grants[i].Before(e.limiter.grants[i-1]), "grants are non-decreasing")
		}
	}
}
