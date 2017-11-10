package times

import (
	"io/ioutil"
	"os"
	"testing"
	"time"
)

type timeRange struct {
	start time.Time
	end   time.Time
}

func newInterval(t time.Time, dur time.Duration) timeRange {
	return timeRange{start: t.Add(-dur), end: t.Add(dur)}
}

func (t timeRange) Contains(findTime time.Time) bool {
	return !findTime.Before(t.start) && !findTime.After(t.end)
}

// creates a file and cleans it up after the test is run.
func fileTest(t testing.TB, testFunc func(f *os.File)) {
	f, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	testFunc(f)
}

func timespecTest(ts Timespec, r timeRange, t testing.TB) {
	if !r.Contains(ts.AccessTime()) {
		t.Errorf("expected %s to be in range: %s\n", ts.AccessTime(), r.start)
	}

	if !r.Contains(ts.ModTime()) {
		t.Errorf("expected %s to be in range: %s\n", ts.ModTime(), r.start)
	}

	if ts.HasChangeTime() && !r.Contains(ts.ChangeTime()) {
		t.Errorf("expected %s to be in range: %s\n", ts.ChangeTime(), r.start)
	}

	if ts.HasBirthTime() && !r.Contains(ts.BirthTime()) {
		t.Errorf("expected %s to be in range: %s\n", ts.BirthTime(), r.start)
	}
}
