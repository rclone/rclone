package accounting

import (
	"context"
	"sync"
	"time"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/rc"
	"github.com/pkg/errors"
	"golang.org/x/time/rate"
)

// Globals
var (
	tokenBucketMu     sync.Mutex // protects the token bucket variables
	tokenBucket       *rate.Limiter
	prevTokenBucket   = tokenBucket
	bwLimitToggledOff = false
	currLimitMu       sync.Mutex // protects changes to the timeslot
	currLimit         fs.BwTimeSlot
)

const maxBurstSize = 4 * 1024 * 1024 // must be bigger than the biggest request

// make a new empty token bucket with the bandwidth given
func newTokenBucket(bandwidth fs.SizeSuffix) *rate.Limiter {
	newTokenBucket := rate.NewLimiter(rate.Limit(bandwidth), maxBurstSize)
	// empty the bucket
	err := newTokenBucket.WaitN(context.Background(), maxBurstSize)
	if err != nil {
		fs.Errorf(nil, "Failed to empty token bucket: %v", err)
	}
	return newTokenBucket
}

// StartTokenBucket starts the token bucket if necessary
func StartTokenBucket() {
	currLimitMu.Lock()
	currLimit := fs.Config.BwLimit.LimitAt(time.Now())
	currLimitMu.Unlock()

	if currLimit.Bandwidth > 0 {
		tokenBucket = newTokenBucket(currLimit.Bandwidth)
		fs.Infof(nil, "Starting bandwidth limiter at %vBytes/s", &currLimit.Bandwidth)

		// Start the SIGUSR2 signal handler to toggle bandwidth.
		// This function does nothing in windows systems.
		startSignalHandler()
	}
}

// StartTokenTicker creates a ticker to update the bandwidth limiter every minute.
func StartTokenTicker() {
	// If the timetable has a single entry or was not specified, we don't need
	// a ticker to update the bandwidth.
	if len(fs.Config.BwLimit) <= 1 {
		return
	}

	ticker := time.NewTicker(time.Minute)
	go func() {
		for range ticker.C {
			limitNow := fs.Config.BwLimit.LimitAt(time.Now())
			currLimitMu.Lock()

			if currLimit.Bandwidth != limitNow.Bandwidth {
				tokenBucketMu.Lock()

				// If bwlimit is toggled off, the change should only
				// become active on the next toggle, which causes
				// an exchange of tokenBucket <-> prevTokenBucket
				var targetBucket **rate.Limiter
				if bwLimitToggledOff {
					targetBucket = &prevTokenBucket
				} else {
					targetBucket = &tokenBucket
				}

				// Set new bandwidth. If unlimited, set tokenbucket to nil.
				if limitNow.Bandwidth > 0 {
					*targetBucket = newTokenBucket(limitNow.Bandwidth)
					if bwLimitToggledOff {
						fs.Logf(nil, "Scheduled bandwidth change. "+
							"Limit will be set to %vBytes/s when toggled on again.", &limitNow.Bandwidth)
					} else {
						fs.Logf(nil, "Scheduled bandwidth change. Limit set to %vBytes/s", &limitNow.Bandwidth)
					}
				} else {
					*targetBucket = nil
					fs.Logf(nil, "Scheduled bandwidth change. Bandwidth limits disabled")
				}

				currLimit = limitNow
				tokenBucketMu.Unlock()
			}
			currLimitMu.Unlock()
		}
	}()
}

// limitBandwith sleeps for the correct amount of time for the passage
// of n bytes according to the current bandwidth limit
func limitBandwidth(n int) {
	tokenBucketMu.Lock()

	// Limit the transfer speed if required
	if tokenBucket != nil {
		err := tokenBucket.WaitN(context.Background(), n)
		if err != nil {
			fs.Errorf(nil, "Token bucket error: %v", err)
		}
	}

	tokenBucketMu.Unlock()
}

// SetBwLimit sets the current bandwidth limit
func SetBwLimit(bandwidth fs.SizeSuffix) {
	tokenBucketMu.Lock()
	defer tokenBucketMu.Unlock()
	if bandwidth > 0 {
		tokenBucket = newTokenBucket(bandwidth)
		fs.Logf(nil, "Bandwidth limit set to %v", bandwidth)
	} else {
		tokenBucket = nil
		fs.Logf(nil, "Bandwidth limit reset to unlimited")
	}
}

// Remote control for the token bucket
func init() {
	rc.Add(rc.Call{
		Path: "core/bwlimit",
		Fn: func(in rc.Params) (out rc.Params, err error) {
			ibwlimit, ok := in["rate"]
			if !ok {
				return out, errors.Errorf("parameter rate not found")
			}
			bwlimit, ok := ibwlimit.(string)
			if !ok {
				return out, errors.Errorf("value must be string rate=%v", ibwlimit)
			}
			var bws fs.BwTimetable
			err = bws.Set(bwlimit)
			if err != nil {
				return out, errors.Wrap(err, "bad bwlimit")
			}
			if len(bws) != 1 {
				return out, errors.New("need exactly 1 bandwidth setting")
			}
			bw := bws[0]
			SetBwLimit(bw.Bandwidth)
			return rc.Params{"rate": bw.Bandwidth.String()}, nil
		},
		Title: "Set the bandwidth limit.",
		Help: `
This sets the bandwidth limit to that passed in.

Eg

    rclone rc core/bwlimit rate=1M
    rclone rc core/bwlimit rate=off

The format of the parameter is exactly the same as passed to --bwlimit
except only one bandwidth may be specified.
`,
	})
}
