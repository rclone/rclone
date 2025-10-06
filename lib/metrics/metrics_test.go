package metrics

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	libhttp "github.com/rclone/rclone/lib/http"
)

func TestHandlerServesMetrics(t *testing.T) {
	require.NoError(t, Init(context.Background()))

	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	Handler().ServeHTTP(rr, req)

	require.Equal(t, 200, rr.Code)
	require.Contains(t, rr.Body.String(), "rclone_bytes_transferred_total")
}

func TestStartStandalone(t *testing.T) {
	opt.HTTP = libhttp.DefaultCfg()
	opt.HTTP.ListenAddr = []string{"127.0.0.1:0"}
	opt.Auth = libhttp.DefaultAuthCfg()
	opt.Template = libhttp.DefaultTemplateCfg()

	srv, err := StartStandalone(context.Background())
	if err != nil {
		if strings.Contains(err.Error(), "operation not permitted") {
			t.Skipf("skipping standalone server test: %v", err)
		}
		require.NoError(t, err)
	}
	require.NotNil(t, srv)

	urls := URLs()
	require.NotEmpty(t, urls)

	require.NoError(t, srv.Shutdown())
	srv.Wait()
}
