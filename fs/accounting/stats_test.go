package accounting

import (
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/stretchr/testify/assert"
)

func TestETA(t *testing.T) {
	for _, test := range []struct {
		size, total int64
		rate        float64
		wantETA     time.Duration
		wantOK      bool
		wantString  string
	}{
		// Custom String Cases
		{size: 0, total: 365 * 86400, rate: 1.0, wantETA: 365 * 86400 * time.Second, wantOK: true, wantString: "1y"},
		{size: 0, total: 7 * 86400, rate: 1.0, wantETA: 7 * 86400 * time.Second, wantOK: true, wantString: "1w"},
		{size: 0, total: 1 * 86400, rate: 1.0, wantETA: 1 * 86400 * time.Second, wantOK: true, wantString: "1d"},
		{size: 0, total: 1110 * 86400, rate: 1.0, wantETA: 1110 * 86400 * time.Second, wantOK: true, wantString: "3y2w1d"},
		{size: 0, total: 15 * 86400, rate: 1.0, wantETA: 15 * 86400 * time.Second, wantOK: true, wantString: "2w1d"},
		// Composite Custom String Cases
		{size: 0, total: 1.5 * 86400, rate: 1.0, wantETA: 1.5 * 86400 * time.Second, wantOK: true, wantString: "1d12h"},
		{size: 0, total: 95000, rate: 1.0, wantETA: 95000 * time.Second, wantOK: true, wantString: "1d2h23m20s"},
		// Standard Duration String Cases
		{size: 0, total: 100, rate: 1.0, wantETA: 100 * time.Second, wantOK: true, wantString: "1m40s"},
		{size: 50, total: 100, rate: 1.0, wantETA: 50 * time.Second, wantOK: true, wantString: "50s"},
		{size: 100, total: 100, rate: 1.0, wantETA: 0 * time.Second, wantOK: true, wantString: "0s"},
		// No String Cases
		{size: -1, total: 100, rate: 1.0, wantETA: 0, wantOK: false, wantString: "-"},
		{size: 200, total: 100, rate: 1.0, wantETA: 0, wantOK: false, wantString: "-"},
		{size: 10, total: -1, rate: 1.0, wantETA: 0, wantOK: false, wantString: "-"},
		{size: 10, total: 20, rate: 0.0, wantETA: 0, wantOK: false, wantString: "-"},
		{size: 10, total: 20, rate: -1.0, wantETA: 0, wantOK: false, wantString: "-"},
		{size: 0, total: 0, rate: 1.0, wantETA: 0, wantOK: false, wantString: "-"},
	} {
		t.Run(fmt.Sprintf("size=%d/total=%d/rate=%f", test.size, test.total, test.rate), func(t *testing.T) {
			gotETA, gotOK := eta(test.size, test.total, test.rate)
			assert.Equal(t, test.wantETA, gotETA)
			assert.Equal(t, test.wantOK, gotOK)
			gotString := etaString(test.size, test.total, test.rate)
			assert.Equal(t, test.wantString, gotString)
		})
	}
}

func TestPercentage(t *testing.T) {
	assert.Equal(t, percent(0, 1000), "0%")
	assert.Equal(t, percent(1, 1000), "0%")
	assert.Equal(t, percent(9, 1000), "1%")
	assert.Equal(t, percent(500, 1000), "50%")
	assert.Equal(t, percent(1000, 1000), "100%")
	assert.Equal(t, percent(1E8, 1E9), "10%")
	assert.Equal(t, percent(1E8, 1E9), "10%")
	assert.Equal(t, percent(0, 0), "-")
	assert.Equal(t, percent(100, -100), "-")
	assert.Equal(t, percent(-100, 100), "-")
	assert.Equal(t, percent(-100, -100), "-")
}

func TestStatsError(t *testing.T) {
	s := NewStats()
	assert.Equal(t, int64(0), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.False(t, s.HadRetryError())
	assert.Equal(t, time.Time{}, s.RetryAfter())
	assert.Equal(t, nil, s.GetLastError())
	assert.False(t, s.Errored())

	t0 := time.Now()
	t1 := t0.Add(time.Second)

	s.Error(nil)
	assert.Equal(t, int64(0), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.False(t, s.HadRetryError())
	assert.Equal(t, time.Time{}, s.RetryAfter())
	assert.Equal(t, nil, s.GetLastError())
	assert.False(t, s.Errored())

	s.Error(io.EOF)
	assert.Equal(t, int64(1), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.True(t, s.HadRetryError())
	assert.Equal(t, time.Time{}, s.RetryAfter())
	assert.Equal(t, io.EOF, s.GetLastError())
	assert.True(t, s.Errored())

	e := fserrors.ErrorRetryAfter(t0)
	s.Error(e)
	assert.Equal(t, int64(2), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.True(t, s.HadRetryError())
	assert.Equal(t, t0, s.RetryAfter())
	assert.Equal(t, e, s.GetLastError())

	err := errors.Wrap(fserrors.ErrorRetryAfter(t1), "potato")
	s.Error(err)
	assert.Equal(t, int64(3), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.True(t, s.HadRetryError())
	assert.Equal(t, t1, s.RetryAfter())
	assert.Equal(t, t1, fserrors.RetryAfterErrorTime(err))

	s.Error(fserrors.FatalError(io.EOF))
	assert.Equal(t, int64(4), s.GetErrors())
	assert.True(t, s.HadFatalError())
	assert.True(t, s.HadRetryError())
	assert.Equal(t, t1, s.RetryAfter())

	s.ResetErrors()
	assert.Equal(t, int64(0), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.False(t, s.HadRetryError())
	assert.Equal(t, time.Time{}, s.RetryAfter())
	assert.Equal(t, nil, s.GetLastError())
	assert.False(t, s.Errored())

	s.Error(fserrors.NoRetryError(io.EOF))
	assert.Equal(t, int64(1), s.GetErrors())
	assert.False(t, s.HadFatalError())
	assert.False(t, s.HadRetryError())
	assert.Equal(t, time.Time{}, s.RetryAfter())
}

func TestStatsTotalDuration(t *testing.T) {
	time1 := time.Now().Add(-40 * time.Second)
	time2 := time1.Add(10 * time.Second)
	time3 := time2.Add(10 * time.Second)
	time4 := time3.Add(10 * time.Second)
	s := NewStats()
	s.AddTransfer(&Transfer{
		startedAt:   time2,
		completedAt: time3,
	})
	s.AddTransfer(&Transfer{
		startedAt:   time2,
		completedAt: time2.Add(time.Second),
	})
	s.AddTransfer(&Transfer{
		startedAt:   time1,
		completedAt: time3,
	})
	s.AddTransfer(&Transfer{
		startedAt:   time3,
		completedAt: time4,
	})
	s.AddTransfer(&Transfer{
		startedAt: time.Now(),
	})

	time.Sleep(time.Millisecond)

	s.mu.Lock()
	total := s.totalDuration()
	s.mu.Unlock()

	assert.True(t, 30*time.Second < total && total < 31*time.Second, total)
}

func TestStatsTotalDuration2(t *testing.T) {
	time1 := time.Now().Add(-40 * time.Second)
	time2 := time1.Add(10 * time.Second)
	s := NewStats()
	s.AddTransfer(&Transfer{
		startedAt:   time1,
		completedAt: time2,
	})

	s.mu.Lock()
	total := s.totalDuration()
	s.mu.Unlock()

	assert.Equal(t, 10*time.Second, total)
}
