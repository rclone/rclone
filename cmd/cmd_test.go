package cmd

import (
	"context"
	"net"
	"testing"

	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/rcserver"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func freeAddr(t *testing.T) string {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	addr := l.Addr().String()
	require.NoError(t, l.Close())
	return addr
}

func TestStopRemoteControl(t *testing.T) {
	ctx := context.Background()
	rcAddr, metricsAddr := freeAddr(t), freeAddr(t)
	opt := rc.Opt
	opt.Enabled = true
	opt.HTTP.ListenAddr = []string{rcAddr}
	opt.MetricsHTTP.ListenAddr = []string{metricsAddr}

	var err error
	rcServer, err = rcserver.Start(ctx, &opt)
	require.NoError(t, err)
	metricsServer, err = rcserver.MetricsStart(ctx, &opt)
	require.NoError(t, err)
	require.NotNil(t, rcServer)
	require.NotNil(t, metricsServer)

	for _, addr := range []string{rcAddr, metricsAddr} {
		_, err := net.Listen("tcp", addr)
		require.Error(t, err, "%s should be in use while the servers run", addr)
	}

	StopRemoteControl()
	assert.Nil(t, rcServer)
	assert.Nil(t, metricsServer)

	for _, addr := range []string{rcAddr, metricsAddr} {
		l, err := net.Listen("tcp", addr)
		require.NoError(t, err, "%s should be free after StopRemoteControl", addr)
		require.NoError(t, l.Close())
	}
}
