package rcserver

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/rclone/rclone/fs/rc"
	"github.com/stretchr/testify/require"
)

// Run a suite of tests
func testMetricsServer(t *testing.T, tests []testRun, opt *rc.Options) {
	t.Helper()
	ctx := context.Background()
	configfile.Install()
	rcServer, err := newMetricsServer(ctx, opt)
	require.NoError(t, err)
	testURL := rcServer.server.URLs()[0]
	mux := rcServer.server.Router()
	emulateCalls(t, tests, mux, testURL)
}

// return an enabled rc
func newMetricsTestOpt() rc.Options {
	opt := rc.Opt
	opt.MetricsHTTP.ListenAddr = []string{testBindAddress}
	return opt
}

func TestMetrics(t *testing.T) {
	stats := accounting.GlobalStats()
	tests := makeMetricsTestCases(stats)
	opt := newMetricsTestOpt()
	testMetricsServer(t, tests, &opt)

	// Test changing a couple options
	stats.Bytes(500)
	for i := 0; i < 30; i++ {
		require.NoError(t, stats.DeleteFile(context.Background(), 0))
	}
	stats.Errors(2)
	stats.Bytes(324)

	tests = makeMetricsTestCases(stats)
	testMetricsServer(t, tests, &opt)
}

func makeMetricsTestCases(stats *accounting.StatsInfo) (tests []testRun) {
	tests = []testRun{{
		Name:     "Bytes Transferred Metric",
		URL:      "metrics",
		Method:   "GET",
		Status:   http.StatusOK,
		Contains: regexp.MustCompile(fmt.Sprintf("rclone_bytes_transferred_total %d", stats.GetBytes())),
	}, {
		Name:     "Checked Files Metric",
		URL:      "metrics",
		Method:   "GET",
		Status:   http.StatusOK,
		Contains: regexp.MustCompile(fmt.Sprintf("rclone_checked_files_total %d", stats.GetChecks())),
	}, {
		Name:     "Errors Metric",
		URL:      "metrics",
		Method:   "GET",
		Status:   http.StatusOK,
		Contains: regexp.MustCompile(fmt.Sprintf("rclone_errors_total %d", stats.GetErrors())),
	}, {
		Name:     "Deleted Files Metric",
		URL:      "metrics",
		Method:   "GET",
		Status:   http.StatusOK,
		Contains: regexp.MustCompile(fmt.Sprintf("rclone_files_deleted_total %d", stats.GetDeletes())),
	}, {
		Name:     "Files Transferred Metric",
		URL:      "metrics",
		Method:   "GET",
		Status:   http.StatusOK,
		Contains: regexp.MustCompile(fmt.Sprintf("rclone_files_transferred_total %d", stats.GetTransfers())),
	},
	}
	return
}
