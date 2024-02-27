package pacer

import (
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNew(t *testing.T) {
	const expectedRetries = 7
	const expectedConnections = 9
	p := New(RetriesOption(expectedRetries), MaxConnectionsOption(expectedConnections))
	if d, ok := p.calculator.(*Default); ok {
		assert.Equal(t, 10*time.Millisecond, d.minSleep)
		assert.Equal(t, 2*time.Second, d.maxSleep)
		assert.Equal(t, d.minSleep, p.state.SleepTime)
		assert.Equal(t, uint(2), d.decayConstant)
		assert.Equal(t, uint(1), d.attackConstant)
	} else {
		t.Errorf("calculator")
	}
	assert.Equal(t, expectedRetries, p.retries)
	assert.Equal(t, 1, cap(p.pacer))
	assert.Equal(t, 1, len(p.pacer))
	assert.Equal(t, expectedConnections, p.maxConnections)
	assert.Equal(t, expectedConnections, cap(p.connTokens))
	assert.Equal(t, 0, p.state.ConsecutiveRetries)
}

func TestMaxConnections(t *testing.T) {
	p := New()
	p.SetMaxConnections(20)
	assert.Equal(t, 20, p.maxConnections)
	assert.Equal(t, 20, cap(p.connTokens))
	p.SetMaxConnections(0)
	assert.Equal(t, 0, p.maxConnections)
	assert.Nil(t, p.connTokens)
}

func TestDecay(t *testing.T) {
	c := NewDefault(MinSleep(1*time.Microsecond), MaxSleep(1*time.Second))
	for _, test := range []struct {
		in             State
		attackConstant uint
		want           time.Duration
	}{
		{State{SleepTime: 8 * time.Millisecond}, 1, 4 * time.Millisecond},
		{State{SleepTime: 1 * time.Millisecond}, 0, 1 * time.Microsecond},
		{State{SleepTime: 1 * time.Millisecond}, 2, (3 * time.Millisecond) / 4},
		{State{SleepTime: 1 * time.Millisecond}, 3, (7 * time.Millisecond) / 8},
	} {
		c.decayConstant = test.attackConstant
		got := c.Calculate(test.in)
		assert.Equal(t, test.want, got, "test: %+v", test)
	}
}

func TestAttack(t *testing.T) {
	c := NewDefault(MinSleep(1*time.Microsecond), MaxSleep(1*time.Second))
	for _, test := range []struct {
		in             State
		attackConstant uint
		want           time.Duration
	}{
		{State{SleepTime: 1 * time.Millisecond, ConsecutiveRetries: 1}, 1, 2 * time.Millisecond},
		{State{SleepTime: 1 * time.Millisecond, ConsecutiveRetries: 1}, 0, 1 * time.Second},
		{State{SleepTime: 1 * time.Millisecond, ConsecutiveRetries: 1}, 2, (4 * time.Millisecond) / 3},
		{State{SleepTime: 1 * time.Millisecond, ConsecutiveRetries: 1}, 3, (8 * time.Millisecond) / 7},
	} {
		c.attackConstant = test.attackConstant
		got := c.Calculate(test.in)
		assert.Equal(t, test.want, got, "test: %+v", test)
	}
}

func TestSetRetries(t *testing.T) {
	p := New()
	p.SetRetries(18)
	assert.Equal(t, 18, p.retries)
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
	p := New(MaxConnectionsOption(10), CalculatorOption(NewDefault(MinSleep(1*time.Millisecond))))
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
	paceTime := waitForPace(p, 1000*time.Millisecond)
	if paceTime.IsZero() {
		t.Errorf("beginSleep didn't fire")
	} else if paceTime.Sub(startTime) < 0 {
		t.Errorf("pace arrived before returning pace token")
	} else if paceTime.Sub(connTime) < 0 {
		t.Errorf("pace arrived before sending conn token")
	}
}

func TestBeginCallZeroConnections(t *testing.T) {
	p := New(MaxConnectionsOption(0), CalculatorOption(NewDefault(MinSleep(1*time.Millisecond))))
	emptyTokens(p)
	go p.beginCall()
	if !waitForPace(p, 10*time.Millisecond).IsZero() {
		t.Errorf("beginSleep fired too early #1")
	}
	startTime := time.Now()
	p.pacer <- struct{}{}
	time.Sleep(1 * time.Millisecond)
	paceTime := waitForPace(p, 1000*time.Millisecond)
	if paceTime.IsZero() {
		t.Errorf("beginSleep didn't fire")
	} else if paceTime.Sub(startTime) < 0 {
		t.Errorf("pace arrived before returning pace token")
	}
}

func TestDefaultPacer(t *testing.T) {
	c := NewDefault(MinSleep(1*time.Millisecond), MaxSleep(1*time.Second), DecayConstant(2))
	for _, test := range []struct {
		state State
		want  time.Duration
	}{
		{State{SleepTime: 1 * time.Millisecond, ConsecutiveRetries: 1}, 2 * time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 1}, 1 * time.Second},
		{State{SleepTime: (3 * time.Second) / 4, ConsecutiveRetries: 1}, 1 * time.Second},
		{State{SleepTime: 1 * time.Second}, 750 * time.Millisecond},
		{State{SleepTime: 1000 * time.Microsecond}, 1 * time.Millisecond},
		{State{SleepTime: 1200 * time.Microsecond}, 1 * time.Millisecond},
	} {
		got := c.Calculate(test.state)
		assert.Equal(t, test.want, got, "test: %+v", test)
	}

}

func TestAzureIMDSPacer(t *testing.T) {
	c := NewAzureIMDS()
	for _, test := range []struct {
		state State
		want  time.Duration
	}{
		{State{SleepTime: 0, ConsecutiveRetries: 0}, 0},
		{State{SleepTime: 0, ConsecutiveRetries: 1}, 2 * time.Second},
		{State{SleepTime: 2 * time.Second, ConsecutiveRetries: 2}, 6 * time.Second},
		{State{SleepTime: 6 * time.Second, ConsecutiveRetries: 3}, 14 * time.Second},
		{State{SleepTime: 14 * time.Second, ConsecutiveRetries: 4}, 30 * time.Second},
	} {
		got := c.Calculate(test.state)
		assert.Equal(t, test.want, got, "test: %+v", test)
	}
}

func TestGoogleDrivePacer(t *testing.T) {
	// Do lots of times because of the random number!
	for _, test := range []struct {
		state State
		want  time.Duration
	}{
		{State{SleepTime: 1 * time.Millisecond}, 0},
		{State{SleepTime: 10 * time.Millisecond}, 0},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 1}, 1*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 2}, 2*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 3}, 4*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 4}, 8*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 5}, 16*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 6}, 16*time.Second + 500*time.Millisecond},
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 7}, 16*time.Second + 500*time.Millisecond},
	} {
		const n = 1000
		var sum time.Duration
		// measure average time over n cycles
		for i := 0; i < n; i++ {
			c := NewGoogleDrive(MinSleep(1 * time.Millisecond))
			sum += c.Calculate(test.state)
		}
		got := sum / n
		assert.False(t, got < (test.want*9)/10 || got > (test.want*11)/10, "test: %+v, got: %v", test, got)
	}

	const minSleep = 2 * time.Millisecond
	for _, test := range []struct {
		calls int
		want  int
	}{
		{1, 0},
		{9, 0},
		{10, 0},
		{11, 1},
		{12, 2},
	} {
		c := NewGoogleDrive(MinSleep(minSleep), Burst(10))
		count := 0
		for i := 0; i < test.calls; i++ {
			sleep := c.Calculate(State{})
			if sleep != 0 {
				count++
			}
		}
		assert.Equalf(t, test.want, count, "test: %+v, got: %v", test, count)
	}
}

func TestS3Pacer(t *testing.T) {
	c := NewS3(MinSleep(10*time.Millisecond), MaxSleep(1*time.Second), DecayConstant(2))
	for _, test := range []struct {
		state State
		want  time.Duration
	}{
		{State{SleepTime: 0, ConsecutiveRetries: 1}, 10 * time.Millisecond},                     //Things were going ok, we failed once, back off to minSleep
		{State{SleepTime: 10 * time.Millisecond, ConsecutiveRetries: 1}, 20 * time.Millisecond}, //Another fail, double the backoff
		{State{SleepTime: 10 * time.Millisecond}, 0},                                            //Things start going ok when we're at minSleep; should result in no sleep
		{State{SleepTime: 12 * time.Millisecond}, 0},                                            //*near* minsleep and going ok, decay would take below minSleep, should go to 0
		{State{SleepTime: 0}, 0},                                                                //Things have been going ok; not retrying should keep sleep at 0
		{State{SleepTime: 1 * time.Second, ConsecutiveRetries: 1}, 1 * time.Second},             //Check maxSleep is enforced
		{State{SleepTime: (3 * time.Second) / 4, ConsecutiveRetries: 1}, 1 * time.Second},       //Check attack heading to maxSleep doesn't exceed maxSleep
		{State{SleepTime: 1 * time.Second}, 750 * time.Millisecond},                             //Check decay from maxSleep
		{State{SleepTime: 48 * time.Millisecond}, 36 * time.Millisecond},                        //Check simple decay above minSleep
	} {
		got := c.Calculate(test.state)
		assert.Equal(t, test.want, got, "test: %+v", test)
	}
}

func TestEndCall(t *testing.T) {
	p := New(MaxConnectionsOption(5))
	emptyTokens(p)
	p.state.ConsecutiveRetries = 1
	p.endCall(true, nil)
	assert.Equal(t, 1, len(p.connTokens))
	assert.Equal(t, 2, p.state.ConsecutiveRetries)
}

func TestEndCallZeroConnections(t *testing.T) {
	p := New(MaxConnectionsOption(0))
	emptyTokens(p)
	p.state.ConsecutiveRetries = 1
	p.endCall(false, nil)
	assert.Equal(t, 0, len(p.connTokens))
	assert.Equal(t, 0, p.state.ConsecutiveRetries)
}

var errFoo = errors.New("foo")

type dummyPaced struct {
	retry  bool
	called int
	wait   *sync.Cond
}

func (dp *dummyPaced) fn() (bool, error) {
	if dp.wait != nil {
		dp.wait.L.Lock()
		dp.called++
		dp.wait.Wait()
		dp.wait.L.Unlock()
	} else {
		dp.called++
	}
	return dp.retry, errFoo
}

func TestCallFixed(t *testing.T) {
	p := New(CalculatorOption(NewDefault(MinSleep(1*time.Millisecond), MaxSleep(2*time.Millisecond))))

	dp := &dummyPaced{retry: false}
	err := p.call(dp.fn, 10)
	assert.Equal(t, 1, dp.called)
	assert.Equal(t, errFoo, err)
}

func Test_callRetry(t *testing.T) {
	p := New(CalculatorOption(NewDefault(MinSleep(1*time.Millisecond), MaxSleep(2*time.Millisecond))))

	dp := &dummyPaced{retry: true}
	err := p.call(dp.fn, 10)
	assert.Equal(t, 10, dp.called)
	assert.Equal(t, errFoo, err)
}

func TestCall(t *testing.T) {
	p := New(RetriesOption(20), CalculatorOption(NewDefault(MinSleep(1*time.Millisecond), MaxSleep(2*time.Millisecond))))

	dp := &dummyPaced{retry: true}
	err := p.Call(dp.fn)
	assert.Equal(t, 20, dp.called)
	assert.Equal(t, errFoo, err)
}

func TestCallParallel(t *testing.T) {
	p := New(MaxConnectionsOption(3), RetriesOption(1), CalculatorOption(NewDefault(MinSleep(100*time.Microsecond), MaxSleep(1*time.Millisecond))))

	wait := sync.NewCond(&sync.Mutex{})
	funcs := make([]*dummyPaced, 5)
	for i := range funcs {
		dp := &dummyPaced{wait: wait}
		funcs[i] = dp
		go func() {
			assert.Equal(t, errFoo, p.CallNoRetry(dp.fn))
		}()
	}
	time.Sleep(250 * time.Millisecond)
	called := 0
	wait.L.Lock()
	for _, dp := range funcs {
		called += dp.called
	}
	wait.L.Unlock()

	assert.Equal(t, 3, called)
	wait.Broadcast()
	time.Sleep(250 * time.Millisecond)

	called = 0
	wait.L.Lock()
	for _, dp := range funcs {
		called += dp.called
	}
	wait.L.Unlock()

	assert.Equal(t, 5, called)
	wait.Broadcast()
}
