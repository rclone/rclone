package metrics

import (
	"context"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/rclone/rclone/fs"
)

const usageCacheTTL = time.Minute

var (
	usageOnce     sync.Once
	usageInstance *usageCollector
)

type usageCollector struct {
	mu        sync.RWMutex
	trackers  map[string]*trackedFS
	usageDesc *prometheus.Desc
	objects   *prometheus.Desc
	supported *prometheus.Desc
	success   *prometheus.Desc
	lastOK    *prometheus.Desc
}

type trackedFS struct {
	id     string
	ctx    context.Context
	fs     fs.Fs
	remote string
	about  func(context.Context) (*fs.Usage, error)

	mu         sync.Mutex
	lastUsage  *fs.Usage
	lastErr    error
	lastUpdate time.Time
	lastOK     time.Time
	lastErrAt  time.Time
	refCount   int
}

func ensureUsageCollector() *usageCollector {
	usageOnce.Do(func() {
		usageInstance = &usageCollector{
			trackers: make(map[string]*trackedFS),
			usageDesc: prometheus.NewDesc(
				"rclone_backend_usage_bytes",
				"Storage usage information as reported by backend About",
				[]string{"remote", "state"},
				nil,
			),
			objects: prometheus.NewDesc(
				"rclone_backend_objects_total",
				"Number of objects reported by backend About",
				[]string{"remote"},
				nil,
			),
			supported: prometheus.NewDesc(
				"rclone_backend_about_supported",
				"Whether the backend implements the About interface",
				[]string{"remote"},
				nil,
			),
			success: prometheus.NewDesc(
				"rclone_backend_about_last_success",
				"Whether the most recent About refresh succeeded",
				[]string{"remote"},
				nil,
			),
			lastOK: prometheus.NewDesc(
				"rclone_backend_about_last_success_timestamp_seconds",
				"Unix timestamp of the last successful About refresh",
				[]string{"remote"},
				nil,
			),
		}
		MustRegisterCollector(usageInstance)
	})
	return usageInstance
}

func (uc *usageCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- uc.usageDesc
	ch <- uc.objects
	ch <- uc.supported
	ch <- uc.success
	ch <- uc.lastOK
}

func (uc *usageCollector) Collect(ch chan<- prometheus.Metric) {
	// Take a snapshot of the trackers to avoid holding the lock while calling out to backends which may take time.
	uc.mu.RLock()
	trackers := make([]*trackedFS, 0, len(uc.trackers))
	for _, tracker := range uc.trackers {
		trackers = append(trackers, tracker)
	}
	uc.mu.RUnlock()

	now := time.Now()
	for _, tracker := range trackers {
		aboutFn := tracker.about
		if aboutFn == nil {
			ch <- prometheus.MustNewConstMetric(uc.supported, prometheus.GaugeValue, 0, tracker.remote)
			continue
		}

		usage, lastOK, err := tracker.snapshot(now)

		ch <- prometheus.MustNewConstMetric(uc.supported, prometheus.GaugeValue, 1, tracker.remote)
		if lastOK.IsZero() {
			ch <- prometheus.MustNewConstMetric(uc.success, prometheus.GaugeValue, 0, tracker.remote)
		} else if err != nil {
			ch <- prometheus.MustNewConstMetric(uc.success, prometheus.GaugeValue, 0, tracker.remote)
			ch <- prometheus.MustNewConstMetric(uc.lastOK, prometheus.GaugeValue, float64(lastOK.Unix()), tracker.remote)
		} else {
			ch <- prometheus.MustNewConstMetric(uc.success, prometheus.GaugeValue, 1, tracker.remote)
			ch <- prometheus.MustNewConstMetric(uc.lastOK, prometheus.GaugeValue, float64(lastOK.Unix()), tracker.remote)
		}

		if usage == nil {
			continue
		}

		uc.emitUsage(ch, tracker, usage)
	}
}

func (uc *usageCollector) emitUsage(ch chan<- prometheus.Metric, tracker *trackedFS, usage *fs.Usage) {
	emit := func(state string, value *int64) {
		if value == nil {
			return
		}
		ch <- prometheus.MustNewConstMetric(uc.usageDesc, prometheus.GaugeValue, float64(*value), tracker.remote, state)
	}

	emit("total", usage.Total)
	emit("used", usage.Used)
	emit("free", usage.Free)
	emit("trashed", usage.Trashed)
	emit("other", usage.Other)

	if usage.Objects != nil {
		ch <- prometheus.MustNewConstMetric(uc.objects, prometheus.GaugeValue, float64(*usage.Objects), tracker.remote)
	}
}

func (t *trackedFS) snapshot(now time.Time) (*fs.Usage, time.Time, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if now.Sub(t.lastUpdate) > usageCacheTTL {
		usage, err := t.about(t.ctx)
		t.lastUpdate = now
		if err != nil {
			t.lastErr = err
			t.lastErrAt = now
		} else {
			t.lastUsage = usage
			t.lastErr = nil
			t.lastOK = now
		}
	}
	return t.lastUsage, t.lastOK, t.lastErr
}

func (uc *usageCollector) track(ctx context.Context, f fs.Fs) func() {
	key := f.Name()

	uc.mu.Lock()
	defer uc.mu.Unlock()

	tracker, exists := uc.trackers[key]
	if exists {
		tracker.refCount++
	} else {
		tracker = &trackedFS{
			id:       key,
			ctx:      ctx,
			fs:       f,
			remote:   f.Name(),
			refCount: 1,
		}
		tracker.about = f.Features().About
		uc.trackers[key] = tracker
	}

	return func() {
		uc.mu.Lock()
		defer uc.mu.Unlock()

		if tracker, exists := uc.trackers[key]; exists {
			tracker.refCount--
			if tracker.refCount <= 0 {
				delete(uc.trackers, key)
			}
		}
	}
}

// TrackFS starts tracking usage statistics for the given fs.Fs.
// It returns a function which stops tracking when called.
func TrackFS(ctx context.Context, f fs.Fs) func() {
	if !Enabled() {
		return func() {}
	}
	uc := ensureUsageCollector()
	return uc.track(ctx, f)
}
