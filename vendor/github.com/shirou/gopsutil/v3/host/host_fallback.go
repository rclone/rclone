//go:build !darwin && !linux && !freebsd && !openbsd && !solaris && !windows
// +build !darwin,!linux,!freebsd,!openbsd,!solaris,!windows

package host

import (
	"context"

	"github.com/shirou/gopsutil/v3/internal/common"
)

func HostIDWithContext(ctx context.Context) (string, error) {
	return "", common.ErrNotImplementedError
}

func numProcs(ctx context.Context) (uint64, error) {
	return 0, common.ErrNotImplementedError
}

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	return 0, common.ErrNotImplementedError
}

func UptimeWithContext(ctx context.Context) (uint64, error) {
	return 0, common.ErrNotImplementedError
}

func UsersWithContext(ctx context.Context) ([]UserStat, error) {
	return []UserStat{}, common.ErrNotImplementedError
}

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	return "", "", common.ErrNotImplementedError
}

func KernelVersionWithContext(ctx context.Context) (string, error) {
	return "", common.ErrNotImplementedError
}

func PlatformInformationWithContext(ctx context.Context) (string, string, string, error) {
	return "", "", "", common.ErrNotImplementedError
}

func SensorsTemperaturesWithContext(ctx context.Context) ([]TemperatureStat, error) {
	return []TemperatureStat{}, common.ErrNotImplementedError
}

func KernelArch() (string, error) {
	return "", common.ErrNotImplementedError
}
