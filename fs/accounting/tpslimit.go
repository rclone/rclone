package accounting

import (
	"context"

	"github.com/rclone/rclone/fs"
	"golang.org/x/time/rate"
)

var (
	tpsBucket *rate.Limiter // for limiting number of http transactions per second
)

// StartLimitTPS starts the token bucket for transactions per second
// limiting if necessary
func StartLimitTPS(ctx context.Context) {
	ci := fs.GetConfig(ctx)
	if ci.TPSLimit > 0 {
		tpsBurst := ci.TPSLimitBurst
		if tpsBurst < 1 {
			tpsBurst = 1
		}
		tpsBucket = rate.NewLimiter(rate.Limit(ci.TPSLimit), tpsBurst)
		fs.Infof(nil, "Starting transaction limiter: max %g transactions/s with burst %d", ci.TPSLimit, tpsBurst)
	}
}

// LimitTPS limits the number of transactions per second if enabled.
// It should be called once per transaction.
func LimitTPS(ctx context.Context) {
	if tpsBucket != nil {
		tbErr := tpsBucket.Wait(ctx)
		if tbErr != nil && tbErr != context.Canceled {
			fs.Errorf(nil, "HTTP token bucket error: %v", tbErr)
		}
	}
}
