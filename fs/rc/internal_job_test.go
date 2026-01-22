// These tests use the job framework so must be external to the module

package rc_test

import (
	"context"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/jobs"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalPanic(t *testing.T) {
	ctx := context.Background()
	call := rc.Calls.Get("rc/panic")
	assert.NotNil(t, call)
	in := rc.Params{}
	_, out, err := jobs.NewJob(ctx, call.Fn, in)
	require.Error(t, err)
	assert.ErrorContains(t, err, "arbitrary error on input map[]")
	assert.ErrorContains(t, err, "panic received:")
	assert.Equal(t, rc.Params{}, out)
}

func TestInternalFatal(t *testing.T) {
	ctx := context.Background()
	call := rc.Calls.Get("rc/fatal")
	assert.NotNil(t, call)
	in := rc.Params{}
	_, out, err := jobs.NewJob(ctx, call.Fn, in)
	require.Error(t, err)
	assert.ErrorContains(t, err, "arbitrary error on input map[]")
	assert.ErrorContains(t, err, "panic received:")
	assert.ErrorContains(t, err, "fatal error:")
	assert.Equal(t, rc.Params{}, out)
}
