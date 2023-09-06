//go:build darwin && !cgo
// +build darwin,!cgo

package process

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/internal/common"
)

func (p *Process) CwdWithContext(ctx context.Context) (string, error) {
	return "", common.ErrNotImplementedError
}

func (p *Process) ExeWithContext(ctx context.Context) (string, error) {
	out, err := invoke.CommandWithContext(ctx, "lsof", "-p", strconv.Itoa(int(p.Pid)), "-Fpfn")
	if err != nil {
		return "", fmt.Errorf("bad call to lsof: %s", err)
	}
	txtFound := 0
	lines := strings.Split(string(out), "\n")
	for i := 1; i < len(lines); i++ {
		if lines[i] == "ftxt" {
			txtFound++
			if txtFound == 2 {
				return lines[i-1][1:], nil
			}
		}
	}
	return "", fmt.Errorf("missing txt data returned by lsof")
}

func (p *Process) CmdlineWithContext(ctx context.Context) (string, error) {
	r, err := callPsWithContext(ctx, "command", p.Pid, false, false)
	if err != nil {
		return "", err
	}
	return strings.Join(r[0], " "), err
}

func (p *Process) cmdNameWithContext(ctx context.Context) (string, error) {
	r, err := callPsWithContext(ctx, "command", p.Pid, false, true)
	if err != nil {
		return "", err
	}
	if len(r) > 0 && len(r[0]) > 0 {
		return r[0][0], err
	}

	return "", err
}

// CmdlineSliceWithContext returns the command line arguments of the process as a slice with each
// element being an argument. Because of current deficiencies in the way that the command
// line arguments are found, single arguments that have spaces in the will actually be
// reported as two separate items. In order to do something better CGO would be needed
// to use the native darwin functions.
func (p *Process) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	r, err := callPsWithContext(ctx, "command", p.Pid, false, false)
	if err != nil {
		return nil, err
	}
	return r[0], err
}

func (p *Process) NumThreadsWithContext(ctx context.Context) (int32, error) {
	r, err := callPsWithContext(ctx, "utime,stime", p.Pid, true, false)
	if err != nil {
		return 0, err
	}
	return int32(len(r)), nil
}

func (p *Process) TimesWithContext(ctx context.Context) (*cpu.TimesStat, error) {
	r, err := callPsWithContext(ctx, "utime,stime", p.Pid, false, false)
	if err != nil {
		return nil, err
	}

	utime, err := convertCPUTimes(r[0][0])
	if err != nil {
		return nil, err
	}
	stime, err := convertCPUTimes(r[0][1])
	if err != nil {
		return nil, err
	}

	ret := &cpu.TimesStat{
		CPU:    "cpu",
		User:   utime,
		System: stime,
	}
	return ret, nil
}

func (p *Process) MemoryInfoWithContext(ctx context.Context) (*MemoryInfoStat, error) {
	r, err := callPsWithContext(ctx, "rss,vsize,pagein", p.Pid, false, false)
	if err != nil {
		return nil, err
	}
	rss, err := strconv.Atoi(r[0][0])
	if err != nil {
		return nil, err
	}
	vms, err := strconv.Atoi(r[0][1])
	if err != nil {
		return nil, err
	}
	pagein, err := strconv.Atoi(r[0][2])
	if err != nil {
		return nil, err
	}

	ret := &MemoryInfoStat{
		RSS:  uint64(rss) * 1024,
		VMS:  uint64(vms) * 1024,
		Swap: uint64(pagein),
	}

	return ret, nil
}
