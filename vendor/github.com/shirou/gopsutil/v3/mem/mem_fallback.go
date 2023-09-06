//go:build !darwin && !linux && !freebsd && !openbsd && !solaris && !windows && !plan9 && !aix
// +build !darwin,!linux,!freebsd,!openbsd,!solaris,!windows,!plan9,!aix

package mem

import (
	"context"

	"github.com/shirou/gopsutil/v3/internal/common"
)

func VirtualMemory() (*VirtualMemoryStat, error) {
	return VirtualMemoryWithContext(context.Background())
}

func VirtualMemoryWithContext(ctx context.Context) (*VirtualMemoryStat, error) {
	return nil, common.ErrNotImplementedError
}

func SwapMemory() (*SwapMemoryStat, error) {
	return SwapMemoryWithContext(context.Background())
}

func SwapMemoryWithContext(ctx context.Context) (*SwapMemoryStat, error) {
	return nil, common.ErrNotImplementedError
}

func SwapDevices() ([]*SwapDevice, error) {
	return SwapDevicesWithContext(context.Background())
}

func SwapDevicesWithContext(ctx context.Context) ([]*SwapDevice, error) {
	return nil, common.ErrNotImplementedError
}
