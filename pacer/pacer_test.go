package pacer

import (
	"fmt"
	"testing"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/pkg/errors"
)

func TestNew(t *testing.T) {
	const expectedRetries = 7
	fs.Config.LowLevelRetries = expectedRetries
	p := New()
	if p.minSleep != 10*time.Millisecond {
		t.Errorf("minSleep")
	}
	if p.maxSleep != 2*time.Second {
		t.Errorf("maxSleep")
	}
	if p.sleepTime != p.minSleep {
		t.Errorf("sleepTime")
	}
	if p.retries != expectedRetries {
		t.Errorf("retries want %v got %v", expectedRetries, p.retries)
	}
	if p.decayConstant != 2 {
		t.Errorf("decayConstant")
	}
	if p.attackConstant != 1 {
		t.Errorf("attackConstant")
	}
	if cap(p.pacer) != 1 {
		t.Errorf("pacer 1")
	}
	if len(p.pacer) != 1 {
		t.Errorf("pacer 2")
	}
	if fmt.Sprintf("%p", p.calculatePace) != fmt.Sprintf("%p", p.defaultPacer) {
		t.Errorf("calculatePace")
	}
	if p.maxConnections != fs.Config.Checkers+fs.Config.Transfers {
		t.Errorf("maxConnections")
	}
	if cap(p.connTokens) != fs.Config.Checkers+fs.Config.Transfers {
		t.Errorf("connTokens")
	}
	if p.consecutiveRetries != 0 {
		t.Errorf("consecutiveRetries")
	}
}

func TestSetSleep(t *testing.T) {
	p := New().SetSleep(2 * time.Millisecond)
	if p.sleepTime != 2*time.Millisecond {
		t.Errorf("didn't set")
	}
}

func TestGetSleep(t *testing.T) {
	p := New().SetSleep(2 * time.Millisecond)
	if p.GetSleep() != 2*time.Millisecond {
		t.Errorf("didn't get")
	}
}

func TestSetMinSleep(t *testing.T) {
	p := New().SetMinSleep(1 * time.Millisecond)
	if p.minSleep != 1*time.Millisecond {
		t.Errorf("didn't set")
	}
}

func TestSetMaxSleep(t *testing.T) {
	p := New().SetMaxSleep(100 * time.Second)
	if p.maxSleep != 100*time.Second {
		t.Errorf("didn't set")
	}
}

func TestMaxConnections(t *testing.T) {
	p := New().SetMaxConnections(20)
	if p.maxConnections != 20 {
		t.Errorf("maxConnections")
	}
	if cap(p.connTokens) != 20 {
		t.Errorf("connTokens")
	}
	p.SetMaxConnections(0)
	if p.maxConnections != 0 {
		t.Errorf("maxConnections is not 0")
	}
	if p.connTokens != nil {
		t.Errorf("connTokens is not nil")
	}
}

func TestSetDecayConstant(t *testing.T) {
	p := New().SetDecayConstant(17)
	if p.decayConstant != 17 {
		t.Errorf("didn't set")
	}
}

func TestDecay(t *testing.T) {
	p := New().SetMinSleep(time.Microsecond).SetPacer(DefaultPacer).SetMaxSleep(time.Second)
	for _, test := range []struct {
		in             time.Duration
		attackConstant uint
		want           time.Duration
	}{
		{8 * time.Millisecond, 1, 4 * time.Millisecond},
		{1 * time.Millisecond, 0, time.Microsecond},
		{1 * time.Millisecond, 2, (3 * time.Millisecond) / 4},
		{1 * time.Millisecond, 3, (7 * time.Millisecond) / 8},
	} {
		p.sleepTime = test.in
		p.SetDecayConstant(test.attackConstant)
		p.defaultPacer(false)
		got := p.sleepTime
		if got != test.want {
			t.Errorf("bad sleep want %v got %v", test.want, got)
		}
	}
}

func TestSetAttackConstant(t *testing.T) {
	p := New().SetAttackConstant(19)
	if p.attackConstant != 19 {
		t.Errorf("didn't set")
	}
}

func TestAttack(t *testing.T) {
	p := New().SetMinSleep(time.Microsecond).SetPacer(DefaultPacer).SetMaxSleep(time.Second)
	for _, test := range []struct {
		in             time.Duration
		attackConstant uint
		want           time.Duration
	}{
		{1 * time.Millisecond, 1, 2 * time.Millisecond},
		{1 * time.Millisecond, 0, time.Second},
		{1 * time.Millisecond, 2, (4 * time.Millisecond) / 3},
		{1 * time.Millisecond, 3, (8 * time.Millisecond) / 7},
	} {
		p.sleepTime = test.in
		p.SetAttackConstant(test.attackConstant)
		p.defaultPacer(true)
		got := p.sleepTime
		if got != test.want {
			t.Errorf("bad sleep want %v got %v", test.want, got)
		}
	}

}

func TestSetRetries(t *testing.T) {
	p := New().SetRetries(18)
	if p.retries != 18 {
		t.Errorf("didn't set")
	}
}

func TestSetPacer(t *testing.T) {
	p := New().SetPacer(AmazonCloudDrivePacer)
	if fmt.Sprintf("%p", p.calculatePace) != fmt.Sprintf("%p", p.acdPacer) {
		t.Errorf("calculatePace is not acdPacer")
	}
	p.SetPacer(GoogleDrivePacer)
	if fmt.Sprintf("%p", p.calculatePace) != fmt.Sprintf("%p", p.drivePacer) {
		t.Errorf("calculatePace is not drivePacer")
	}
	p.SetPacer(DefaultPacer)
	if fmt.Sprintf("%p", p.calculatePace) != fmt.Sprintf("%p", p.defaultPacer) {
		t.Errorf("calculatePace is not defaultPacer")
	}
}

// emptyTokens empties the pacer of all its tokens
func emptyTokens(p *Pacer) {
	for len(p.pacer) != 0 {
		<-p.pacer
	}
	for len(p.connTokens) != 0 {
		<-p.connTokens
	}
}

// waitForPace waits for duration for the pace to arrive
// returns the time that it arrived or a zero time
func waitForPace(p *Pacer, duration time.Duration) (when time.Time) {
	select {
	case <-time.After(duration):
		return
	case <-p.pacer:
		return time.Now()
	}
}

func TestBeginCall(t *testing.T) {
	p := New().SetMaxConnections(10).SetMinSleep(1 * time.Millisecond)
	emptyTokens(p)
	go p.beginCall()
	if !waitForPace(p, 10*time.Millisecond).IsZero() {
		t.Errorf("beginSleep fired too early #1")
	}
	startTime := time.Now()
	p.pacer <- struct{}{}
	time.Sleep(1 * time.Millisecond)
	connTime := time.Now()
	p.connTokens <- struct{}{}
	time.Sleep(1 * time.Millisecond)
	paceTime := waitForPace(p, 10*time.Millisecond)
	if paceTime.IsZero() {
		t.Errorf("beginSleep didn't fire")
	} else if paceTime.Sub(startTime) < 0 {
		t.Errorf("pace arrived before returning pace token")
	} else if paceTime.Sub(connTime) < 0 {
		t.Errorf("pace arrived before sending conn token")
	}
}

func TestBeginCallZeroConnections(t *testing.T) {
	p := New().SetMaxConnections(0).SetMinSleep(1 * time.Millisecond)
	emptyTokens(p)
	go p.beginCall()
	if !waitForPace(p, 10*time.Millisecond).IsZero() {
		t.Errorf("beginSleep fired too early #1")
	}
	startTime := time.Now()
	p.pacer <- struct{}{}
	time.Sleep(1 * time.Millisecond)
	paceTime := waitForPace(p, 10*time.Millisecond)
	if paceTime.IsZero() {
		t.Errorf("beginSleep didn't fire")
	} else if paceTime.Sub(startTime) < 0 {
		t.Errorf("pace arrived before returning pace token")
	}
}

func TestDefaultPacer(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetPacer(DefaultPacer).SetMaxSleep(time.Second).SetDecayConstant(2)
	for _, test := range []struct {
		in    time.Duration
		retry bool
		want  time.Duration
	}{
		{time.Millisecond, true, 2 * time.Millisecond},
		{time.Second, true, time.Second},
		{(3 * time.Second) / 4, true, time.Second},
		{time.Second, false, 750 * time.Millisecond},
		{1000 * time.Microsecond, false, time.Millisecond},
		{1200 * time.Microsecond, false, time.Millisecond},
	} {
		p.sleepTime = test.in
		p.defaultPacer(test.retry)
		got := p.sleepTime
		if got != test.want {
			t.Errorf("bad sleep want %v got %v", test.want, got)
		}
	}

}

func TestAmazonCloudDrivePacer(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetPacer(AmazonCloudDrivePacer).SetMaxSleep(time.Second).SetDecayConstant(2)
	// Do lots of times because of the random number!
	for _, test := range []struct {
		in                 time.Duration
		consecutiveRetries int
		retry              bool
		want               time.Duration
	}{
		{time.Millisecond, 0, true, time.Millisecond},
		{10 * time.Millisecond, 0, true, time.Millisecond},
		{1 * time.Second, 1, true, 500 * time.Millisecond},
		{1 * time.Second, 2, true, 1 * time.Second},
		{1 * time.Second, 3, true, 2 * time.Second},
		{1 * time.Second, 4, true, 4 * time.Second},
		{1 * time.Second, 5, true, 8 * time.Second},
		{1 * time.Second, 6, true, 16 * time.Second},
		{1 * time.Second, 7, true, 32 * time.Second},
		{1 * time.Second, 8, true, 64 * time.Second},
		{1 * time.Second, 9, true, 128 * time.Second},
		{1 * time.Second, 10, true, 128 * time.Second},
		{1 * time.Second, 11, true, 128 * time.Second},
	} {
		const n = 1000
		var sum time.Duration
		// measure average time over n cycles
		for i := 0; i < n; i++ {
			p.sleepTime = test.in
			p.consecutiveRetries = test.consecutiveRetries
			p.acdPacer(test.retry)
			sum += p.sleepTime
		}
		got := sum / n
		//t.Logf("%+v: got = %v", test, got)
		if got < (test.want*9)/10 || got > (test.want*11)/10 {
			t.Fatalf("%+v: bad sleep want %v+/-10%% got %v", test, test.want, got)
		}
	}
}

func TestGoogleDrivePacer(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetPacer(GoogleDrivePacer).SetMaxSleep(time.Second).SetDecayConstant(2)
	// Do lots of times because of the random number!
	for _, test := range []struct {
		in                 time.Duration
		consecutiveRetries int
		retry              bool
		want               time.Duration
	}{
		{time.Millisecond, 0, true, time.Millisecond},
		{10 * time.Millisecond, 0, true, time.Millisecond},
		{1 * time.Second, 1, true, 1*time.Second + 500*time.Millisecond},
		{1 * time.Second, 2, true, 2*time.Second + 500*time.Millisecond},
		{1 * time.Second, 3, true, 4*time.Second + 500*time.Millisecond},
		{1 * time.Second, 4, true, 8*time.Second + 500*time.Millisecond},
		{1 * time.Second, 5, true, 16*time.Second + 500*time.Millisecond},
		{1 * time.Second, 6, true, 16*time.Second + 500*time.Millisecond},
		{1 * time.Second, 7, true, 16*time.Second + 500*time.Millisecond},
	} {
		const n = 1000
		var sum time.Duration
		// measure average time over n cycles
		for i := 0; i < n; i++ {
			p.sleepTime = test.in
			p.consecutiveRetries = test.consecutiveRetries
			p.drivePacer(test.retry)
			sum += p.sleepTime
		}
		got := sum / n
		//t.Logf("%+v: got = %v", test, got)
		if got < (test.want*9)/10 || got > (test.want*11)/10 {
			t.Fatalf("%+v: bad sleep want %v+/-10%% got %v", test, test.want, got)
		}
	}
}

func TestEndCall(t *testing.T) {
	p := New().SetMaxConnections(5)
	emptyTokens(p)
	p.consecutiveRetries = 1
	p.endCall(true)
	if len(p.connTokens) != 1 {
		t.Errorf("Expecting 1 token")
	}
	if p.consecutiveRetries != 2 {
		t.Errorf("Bad consecutive retries")
	}
}

func TestEndCallZeroConnections(t *testing.T) {
	p := New().SetMaxConnections(0)
	emptyTokens(p)
	p.consecutiveRetries = 1
	p.endCall(false)
	if len(p.connTokens) != 0 {
		t.Errorf("Expecting 0 token")
	}
	if p.consecutiveRetries != 0 {
		t.Errorf("Bad consecutive retries")
	}
}

var errFoo = errors.New("foo")

type dummyPaced struct {
	retry  bool
	called int
}

func (dp *dummyPaced) fn() (bool, error) {
	dp.called++
	return dp.retry, errFoo
}

func Test_callNoRetry(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetMaxSleep(2 * time.Millisecond)

	dp := &dummyPaced{retry: false}
	err := p.call(dp.fn, 10)
	if dp.called != 1 {
		t.Errorf("called want %d got %d", 1, dp.called)
	}
	if err != errFoo {
		t.Errorf("err want %v got %v", errFoo, err)
	}
}

func Test_callRetry(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetMaxSleep(2 * time.Millisecond)

	dp := &dummyPaced{retry: true}
	err := p.call(dp.fn, 10)
	if dp.called != 10 {
		t.Errorf("called want %d got %d", 10, dp.called)
	}
	if err == errFoo {
		t.Errorf("err didn't want %v got %v", errFoo, err)
	}
	_, ok := err.(fs.Retrier)
	if !ok {
		t.Errorf("didn't return a retry error")
	}
}

func TestCall(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetMaxSleep(2 * time.Millisecond).SetRetries(20)

	dp := &dummyPaced{retry: true}
	err := p.Call(dp.fn)
	if dp.called != 20 {
		t.Errorf("called want %d got %d", 20, dp.called)
	}
	_, ok := err.(fs.Retrier)
	if !ok {
		t.Errorf("didn't return a retry error")
	}
}

func TestCallNoRetry(t *testing.T) {
	p := New().SetMinSleep(time.Millisecond).SetMaxSleep(2 * time.Millisecond).SetRetries(20)

	dp := &dummyPaced{retry: true}
	err := p.CallNoRetry(dp.fn)
	if dp.called != 1 {
		t.Errorf("called want %d got %d", 1, dp.called)
	}
	_, ok := err.(fs.Retrier)
	if !ok {
		t.Errorf("didn't return a retry error")
	}
}
