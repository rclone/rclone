//go:build darwin || freebsd || openbsd
// +build darwin freebsd openbsd

package host

import (
	"context"
	"sync/atomic"

	"golang.org/x/sys/unix"
)

// cachedBootTime must be accessed via atomic.Load/StoreUint64
var cachedBootTime uint64

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	t := atomic.LoadUint64(&cachedBootTime)
	if t != 0 {
		return t, nil
	}
	tv, err := unix.SysctlTimeval("kern.boottime")
	if err != nil {
		return 0, err
	}

	atomic.StoreUint64(&cachedBootTime, uint64(tv.Sec))

	return uint64(tv.Sec), nil
}

func UptimeWithContext(ctx context.Context) (uint64, error) {
	boot, err := BootTimeWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return timeSince(boot), nil
}
