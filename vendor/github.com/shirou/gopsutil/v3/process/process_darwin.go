//go:build darwin
// +build darwin

package process

import (
	"context"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/tklauser/go-sysconf"
	"golang.org/x/sys/unix"

	"github.com/shirou/gopsutil/v3/internal/common"
	"github.com/shirou/gopsutil/v3/net"
)

// copied from sys/sysctl.h
const (
	CTLKern          = 1  // "high kernel": proc, limits
	KernProc         = 14 // struct: process entries
	KernProcPID      = 1  // by process id
	KernProcProc     = 8  // only return procs
	KernProcAll      = 0  // everything
	KernProcPathname = 12 // path to executable
)

var clockTicks = 100 // default value

func init() {
	clkTck, err := sysconf.Sysconf(sysconf.SC_CLK_TCK)
	// ignore errors
	if err == nil {
		clockTicks = int(clkTck)
	}
}

type _Ctype_struct___0 struct {
	Pad uint64
}

func pidsWithContext(ctx context.Context) ([]int32, error) {
	var ret []int32

	kprocs, err := unix.SysctlKinfoProcSlice("kern.proc.all")
	if err != nil {
		return ret, err
	}

	for _, proc := range kprocs {
		ret = append(ret, int32(proc.Proc.P_pid))
	}

	return ret, nil
}

func (p *Process) PpidWithContext(ctx context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Eproc.Ppid, nil
}

func (p *Process) NameWithContext(ctx context.Context) (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}

	name := common.ByteToString(k.Proc.P_comm[:])

	if len(name) >= 15 {
		cmdName, err := p.cmdNameWithContext(ctx)
		if err != nil {
			return "", err
		}
		if len(cmdName) > 0 {
			extendedName := filepath.Base(cmdName)
			if strings.HasPrefix(extendedName, p.name) {
				name = extendedName
			} else {
				name = cmdName
			}
		}
	}

	return name, nil
}

func (p *Process) createTimeWithContext(ctx context.Context) (int64, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Proc.P_starttime.Sec*1000 + int64(k.Proc.P_starttime.Usec)/1000, nil
}

func (p *Process) StatusWithContext(ctx context.Context) ([]string, error) {
	r, err := callPsWithContext(ctx, "state", p.Pid, false, false)
	if err != nil {
		return []string{""}, err
	}
	status := convertStatusChar(r[0][0][0:1])
	return []string{status}, err
}

func (p *Process) ForegroundWithContext(ctx context.Context) (bool, error) {
	// see https://github.com/shirou/gopsutil/issues/596#issuecomment-432707831 for implementation details
	pid := p.Pid
	out, err := invoke.CommandWithContext(ctx, "ps", "-o", "stat=", "-p", strconv.Itoa(int(pid)))
	if err != nil {
		return false, err
	}
	return strings.IndexByte(string(out), '+') != -1, nil
}

func (p *Process) UidsWithContext(ctx context.Context) ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	// See: http://unix.superglobalmegacorp.com/Net2/newsrc/sys/ucred.h.html
	userEffectiveUID := int32(k.Eproc.Ucred.Uid)

	return []int32{userEffectiveUID}, nil
}

func (p *Process) GidsWithContext(ctx context.Context) ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	gids := make([]int32, 0, 3)
	gids = append(gids, int32(k.Eproc.Pcred.P_rgid), int32(k.Eproc.Pcred.P_rgid), int32(k.Eproc.Pcred.P_svgid))

	return gids, nil
}

func (p *Process) GroupsWithContext(ctx context.Context) ([]int32, error) {
	return nil, common.ErrNotImplementedError
	// k, err := p.getKProc()
	// if err != nil {
	// 	return nil, err
	// }

	// groups := make([]int32, k.Eproc.Ucred.Ngroups)
	// for i := int16(0); i < k.Eproc.Ucred.Ngroups; i++ {
	// 	groups[i] = int32(k.Eproc.Ucred.Groups[i])
	// }

	// return groups, nil
}

func (p *Process) TerminalWithContext(ctx context.Context) (string, error) {
	return "", common.ErrNotImplementedError
	/*
		k, err := p.getKProc()
		if err != nil {
			return "", err
		}

		ttyNr := uint64(k.Eproc.Tdev)
		termmap, err := getTerminalMap()
		if err != nil {
			return "", err
		}

		return termmap[ttyNr], nil
	*/
}

func (p *Process) NiceWithContext(ctx context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}
	return int32(k.Proc.P_nice), nil
}

func (p *Process) IOCountersWithContext(ctx context.Context) (*IOCountersStat, error) {
	return nil, common.ErrNotImplementedError
}

func convertCPUTimes(s string) (ret float64, err error) {
	var t int
	var _tmp string
	if strings.Contains(s, ":") {
		_t := strings.Split(s, ":")
		switch len(_t) {
		case 3:
			hour, err := strconv.Atoi(_t[0])
			if err != nil {
				return ret, err
			}
			t += hour * 60 * 60 * clockTicks

			mins, err := strconv.Atoi(_t[1])
			if err != nil {
				return ret, err
			}
			t += mins * 60 * clockTicks
			_tmp = _t[2]
		case 2:
			mins, err := strconv.Atoi(_t[0])
			if err != nil {
				return ret, err
			}
			t += mins * 60 * clockTicks
			_tmp = _t[1]
		case 1, 0:
			_tmp = s
		default:
			return ret, fmt.Errorf("wrong cpu time string")
		}
	} else {
		_tmp = s
	}

	_t := strings.Split(_tmp, ".")
	if err != nil {
		return ret, err
	}
	h, err := strconv.Atoi(_t[0])
	t += h * clockTicks
	h, err = strconv.Atoi(_t[1])
	t += h
	return float64(t) / float64(clockTicks), nil
}

func (p *Process) ChildrenWithContext(ctx context.Context) ([]*Process, error) {
	pids, err := common.CallPgrepWithContext(ctx, invoke, p.Pid)
	if err != nil {
		return nil, err
	}
	ret := make([]*Process, 0, len(pids))
	for _, pid := range pids {
		np, err := NewProcessWithContext(ctx, pid)
		if err != nil {
			return nil, err
		}
		ret = append(ret, np)
	}
	return ret, nil
}

func (p *Process) ConnectionsWithContext(ctx context.Context) ([]net.ConnectionStat, error) {
	return net.ConnectionsPidWithContext(ctx, "all", p.Pid)
}

func (p *Process) ConnectionsMaxWithContext(ctx context.Context, max int) ([]net.ConnectionStat, error) {
	return net.ConnectionsPidMaxWithContext(ctx, "all", p.Pid, max)
}

func ProcessesWithContext(ctx context.Context) ([]*Process, error) {
	out := []*Process{}

	pids, err := PidsWithContext(ctx)
	if err != nil {
		return out, err
	}

	for _, pid := range pids {
		p, err := NewProcessWithContext(ctx, pid)
		if err != nil {
			continue
		}
		out = append(out, p)
	}

	return out, nil
}

// Returns a proc as defined here:
// http://unix.superglobalmegacorp.com/Net2/newsrc/sys/kinfo_proc.h.html
func (p *Process) getKProc() (*unix.KinfoProc, error) {
	return unix.SysctlKinfoProc("kern.proc.pid", int(p.Pid))
}

// call ps command.
// Return value deletes Header line(you must not input wrong arg).
// And splited by Space. Caller have responsibility to manage.
// If passed arg pid is 0, get information from all process.
func callPsWithContext(ctx context.Context, arg string, pid int32, threadOption bool, nameOption bool) ([][]string, error) {
	var cmd []string
	if pid == 0 { // will get from all processes.
		cmd = []string{"-ax", "-o", arg}
	} else if threadOption {
		cmd = []string{"-x", "-o", arg, "-M", "-p", strconv.Itoa(int(pid))}
	} else {
		cmd = []string{"-x", "-o", arg, "-p", strconv.Itoa(int(pid))}
	}
	if nameOption {
		cmd = append(cmd, "-c")
	}
	out, err := invoke.CommandWithContext(ctx, "ps", cmd...)
	if err != nil {
		return [][]string{}, err
	}
	lines := strings.Split(string(out), "\n")

	var ret [][]string
	for _, l := range lines[1:] {
		var lr []string
		if nameOption {
			lr = append(lr, l)
		} else {
			for _, r := range strings.Split(l, " ") {
				if r == "" {
					continue
				}
				lr = append(lr, strings.TrimSpace(r))
			}
		}
		if len(lr) != 0 {
			ret = append(ret, lr)
		}
	}

	return ret, nil
}
