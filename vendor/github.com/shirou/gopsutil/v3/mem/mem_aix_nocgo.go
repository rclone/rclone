//go:build aix && !cgo
// +build aix,!cgo

package mem

import (
	"context"
	"regexp"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/internal/common"
)

var whiteSpaces = regexp.MustCompile(`\s+`)

func VirtualMemoryWithContext(ctx context.Context) (*VirtualMemoryStat, error) {
	vmem, swap, err := callSVMon(ctx)
	if err != nil {
		return nil, err
	}
	if vmem.Total == 0 {
		return nil, common.ErrNotImplementedError
	}
	vmem.SwapTotal = swap.Total
	vmem.SwapFree = swap.Free
	return vmem, nil
}

func SwapMemoryWithContext(ctx context.Context) (*SwapMemoryStat, error) {
	_, swap, err := callSVMon(ctx)
	if err != nil {
		return nil, err
	}
	if swap.Total == 0 {
		return nil, common.ErrNotImplementedError
	}
	return swap, nil
}

func callSVMon(ctx context.Context) (*VirtualMemoryStat, *SwapMemoryStat, error) {
	out, err := invoke.CommandWithContext(ctx, "svmon", "-G")
	if err != nil {
		return nil, nil, err
	}

	pagesize := uint64(4096)
	vmem := &VirtualMemoryStat{}
	swap := &SwapMemoryStat{}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.HasPrefix(line, "memory") {
			p := whiteSpaces.Split(line, 7)
			if len(p) > 2 {
				if t, err := strconv.ParseUint(p[1], 10, 64); err == nil {
					vmem.Total = t * pagesize
				}
				if t, err := strconv.ParseUint(p[2], 10, 64); err == nil {
					vmem.Used = t * pagesize
					if vmem.Total > 0 {
						vmem.UsedPercent = 100 * float64(vmem.Used) / float64(vmem.Total)
					}
				}
				if t, err := strconv.ParseUint(p[3], 10, 64); err == nil {
					vmem.Free = t * pagesize
				}
			}
		} else if strings.HasPrefix(line, "pg space") {
			p := whiteSpaces.Split(line, 4)
			if len(p) > 3 {
				if t, err := strconv.ParseUint(p[2], 10, 64); err == nil {
					swap.Total = t * pagesize
				}
				if t, err := strconv.ParseUint(p[3], 10, 64); err == nil {
					swap.Free = swap.Total - t*pagesize
				}
			}
			break
		}
	}
	return vmem, swap, nil
}
