package fs

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/stretchr/testify/require"
)

var errFoo = errors.New("foo")

type dummyPaced struct {
	retry  bool
	called int
	wait   *sync.Cond
}

func (dp *dummyPaced) fn() (bool, error) {
	if dp.wait != nil {
		dp.wait.L.Lock()
		dp.wait.Wait()
		dp.wait.L.Unlock()
	}
	dp.called++
	return dp.retry, errFoo
}

func TestPacerCall(t *testing.T) {
	ctx := context.Background()
	config := GetConfig(ctx)
	expectedCalled := config.LowLevelRetries
	if expectedCalled == 0 {
		ctx, config = AddConfig(ctx)
		expectedCalled = 20
		config.LowLevelRetries = expectedCalled
	}
	p := NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.Call(dp.fn)
	require.Equal(t, expectedCalled, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}

func TestPacerCallNoRetry(t *testing.T) {
	p := NewPacer(context.Background(), pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.CallNoRetry(dp.fn)
	require.Equal(t, 1, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}
