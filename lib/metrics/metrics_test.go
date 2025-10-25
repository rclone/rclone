package metrics_test

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/metrics"
)

func TestHandlerServesMetrics(t *testing.T) {
	require.NoError(t, metrics.Init(context.Background()))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	metrics.Handler().ServeHTTP(rr, req)

	require.Equal(t, 200, rr.Code)
	require.Contains(t, rr.Body.String(), "rclone_bytes_transferred_total")
}

func TestStartStandalone(t *testing.T) {
	metrics.Opt.HTTP = libhttp.DefaultCfg()
	metrics.Opt.HTTP.ListenAddr = []string{"127.0.0.1:0"}
	metrics.Opt.Auth = libhttp.DefaultAuthCfg()
	metrics.Opt.Template = libhttp.DefaultTemplateCfg()

	srv, err := metrics.StartStandalone(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping standalone server test: %v", err)
		}
		require.NoError(t, err)
	}
	require.NotNil(t, srv)

	urls := metrics.URLs()
	require.NotEmpty(t, urls)

	require.NoError(t, srv.Shutdown())
	srv.Wait()
}
