//go:build darwin
// +build darwin

package cpu

import (
	"context"
	"strconv"
	"strings"

	"github.com/shoenig/go-m1cpu"
	"github.com/tklauser/go-sysconf"
	"golang.org/x/sys/unix"
)

// sys/resource.h
const (
	CPUser    = 0
	cpNice    = 1
	cpSys     = 2
	cpIntr    = 3
	cpIdle    = 4
	cpUStates = 5
)

// default value. from time.h
var ClocksPerSec = float64(128)

func init() {
	clkTck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	// ignore errors
	if err == nil {
		ClocksPerSec = float64(clkTck)
	}
}

func Times(percpu bool) ([]TimesStat, error) {
	return TimesWithContext(context.Background(), percpu)
}

func TimesWithContext(ctx context.Context, percpu bool) ([]TimesStat, error) {
	if percpu {
		return perCPUTimes()
	}

	return allCPUTimes()
}

// Returns only one CPUInfoStat on FreeBSD
func Info() ([]InfoStat, error) {
	return InfoWithContext(context.Background())
}

func InfoWithContext(ctx context.Context) ([]InfoStat, error) {
	var ret []InfoStat

	c := InfoStat{}
	c.ModelName, _ = unix.Sysctl("machdep.cpu.brand_string")
	family, _ := unix.SysctlUint32("machdep.cpu.family")
	c.Family = strconv.FormatUint(uint64(family), 10)
	model, _ := unix.SysctlUint32("machdep.cpu.model")
	c.Model = strconv.FormatUint(uint64(model), 10)
	stepping, _ := unix.SysctlUint32("machdep.cpu.stepping")
	c.Stepping = int32(stepping)
	features, err := unix.Sysctl("machdep.cpu.features")
	if err == nil {
		for _, v := range strings.Fields(features) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	leaf7Features, err := unix.Sysctl("machdep.cpu.leaf7_features")
	if err == nil {
		for _, v := range strings.Fields(leaf7Features) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	extfeatures, err := unix.Sysctl("machdep.cpu.extfeatures")
	if err == nil {
		for _, v := range strings.Fields(extfeatures) {
			c.Flags = append(c.Flags, strings.ToLower(v))
		}
	}
	cores, _ := unix.SysctlUint32("machdep.cpu.core_count")
	c.Cores = int32(cores)
	cacheSize, _ := unix.SysctlUint32("machdep.cpu.cache.size")
	c.CacheSize = int32(cacheSize)
	c.VendorID, _ = unix.Sysctl("machdep.cpu.vendor")

	if m1cpu.IsAppleSilicon() {
		c.Mhz = float64(m1cpu.PCoreHz() / 1_000_000)
	} else {
		// Use the rated frequency of the CPU. This is a static value and does not
		// account for low power or Turbo Boost modes.
		cpuFrequency, err := unix.SysctlUint64("hw.cpufrequency")
		if err == nil {
			c.Mhz = float64(cpuFrequency) / 1000000.0
		}
	}

	return append(ret, c), nil
}

func CountsWithContext(ctx context.Context, logical bool) (int, error) {
	var cpuArgument string
	if logical {
		cpuArgument = "hw.logicalcpu"
	} else {
		cpuArgument = "hw.physicalcpu"
	}

	count, err := unix.SysctlUint32(cpuArgument)
	if err != nil {
		return 0, err
	}

	return int(count), nil
}
