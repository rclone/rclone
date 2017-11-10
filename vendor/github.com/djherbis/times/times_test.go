package times

import (
	"os"
	"testing"
	"time"
)

func TestStat(t *testing.T) {
	fileTest(t, func(f *os.File) {
		ts, err := Stat(f.Name())
		if err != nil {
			t.Error(err.Error())
		}
		timespecTest(ts, newInterval(time.Now(), time.Second), t)
	})
}

func TestGet(t *testing.T) {
	fileTest(t, func(f *os.File) {
		fi, err := os.Stat(f.Name())
		if err != nil {
			t.Error(err.Error())
		}
		timespecTest(Get(fi), newInterval(time.Now(), time.Second), t)
	})
}

func TestStatErr(t *testing.T) {
	_, err := Stat("badfile?")
	if err == nil {
		t.Error("expected an error")
	}
}

func TestCheat(t *testing.T) {
	// not all times are available for all platforms
	// this allows us to get 100% test coverage for platforms which do not have
	// ChangeTime/BirthTime
	var c ctime
	if c.HasChangeTime() {
		c.ChangeTime()
	}

	var b btime
	if b.HasBirthTime() {
		b.BirthTime()
	}

	var nc noctime
	func() {
		if !nc.HasChangeTime() {
			defer func() { recover() }()
		}
		nc.ChangeTime()
	}()

	var nb nobtime
	func() {
		if !nb.HasBirthTime() {
			defer func() { recover() }()
		}
		nb.BirthTime()
	}()
}
