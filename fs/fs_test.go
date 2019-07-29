package fs

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

func TestFeaturesDisable(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(ctx context.Context, src Object, remote string) (Object, error) {
		return nil, nil
	}
	ft.CaseInsensitive = true

	assert.NotNil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	ft.Disable("copy")
	assert.Nil(t, ft.Copy)
	assert.Nil(t, ft.Purge)

	assert.True(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
	ft.Disable("caseinsensitive")
	assert.False(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
}

func TestFeaturesList(t *testing.T) {
	ft := new(Features)
	names := strings.Join(ft.List(), ",")
	assert.True(t, strings.Contains(names, ",Copy,"))
}

func TestFeaturesEnabled(t *testing.T) {
	ft := new(Features)
	ft.CaseInsensitive = true
	ft.Purge = func(ctx context.Context) error { return nil }
	enabled := ft.Enabled()

	flag, ok := enabled["CaseInsensitive"]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, flag, enabled)

	flag, ok = enabled["Purge"]
	assert.Equal(t, true, ok)
	assert.Equal(t, true, flag, enabled)

	flag, ok = enabled["DuplicateFiles"]
	assert.Equal(t, true, ok)
	assert.Equal(t, false, flag, enabled)

	flag, ok = enabled["Copy"]
	assert.Equal(t, true, ok)
	assert.Equal(t, false, flag, enabled)

	assert.Equal(t, len(ft.List()), len(enabled))
}

func TestFeaturesDisableList(t *testing.T) {
	ft := new(Features)
	ft.Copy = func(ctx context.Context, src Object, remote string) (Object, error) {
		return nil, nil
	}
	ft.CaseInsensitive = true

	assert.NotNil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	assert.True(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)

	ft.DisableList([]string{"copy", "caseinsensitive"})

	assert.Nil(t, ft.Copy)
	assert.Nil(t, ft.Purge)
	assert.False(t, ft.CaseInsensitive)
	assert.False(t, ft.DuplicateFiles)
}

// Check it satisfies the interface
var _ pflag.Value = (*Option)(nil)

func TestOption(t *testing.T) {
	d := &Option{
		Name:  "potato",
		Value: SizeSuffix(17 << 20),
	}
	assert.Equal(t, "17M", d.String())
	assert.Equal(t, "SizeSuffix", d.Type())
	err := d.Set("18M")
	assert.NoError(t, err)
	assert.Equal(t, SizeSuffix(18<<20), d.Value)
	err = d.Set("sdfsdf")
	assert.Error(t, err)
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
		dp.wait.Wait()
		dp.wait.L.Unlock()
	}
	dp.called++
	return dp.retry, errFoo
}

func TestPacerCall(t *testing.T) {
	expectedCalled := Config.LowLevelRetries
	if expectedCalled == 0 {
		expectedCalled = 20
		Config.LowLevelRetries = expectedCalled
		defer func() {
			Config.LowLevelRetries = 0
		}()
	}
	p := NewPacer(pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.Call(dp.fn)
	require.Equal(t, expectedCalled, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}

func TestPacerCallNoRetry(t *testing.T) {
	p := NewPacer(pacer.NewDefault(pacer.MinSleep(1*time.Millisecond), pacer.MaxSleep(2*time.Millisecond)))

	dp := &dummyPaced{retry: true}
	err := p.CallNoRetry(dp.fn)
	require.Equal(t, 1, dp.called)
	require.Implements(t, (*fserrors.Retrier)(nil), err)
}
