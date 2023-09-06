//go:build freebsd
// +build freebsd

package process

import (
	"bytes"
	"context"
	"path/filepath"
	"strconv"
	"strings"

	cpu "github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/internal/common"
	net "github.com/shirou/gopsutil/v3/net"
	"golang.org/x/sys/unix"
)

func pidsWithContext(ctx context.Context) ([]int32, error) {
	var ret []int32
	procs, err := ProcessesWithContext(ctx)
	if err != nil {
		return ret, nil
	}

	for _, p := range procs {
		ret = append(ret, p.Pid)
	}

	return ret, nil
}

func (p *Process) PpidWithContext(ctx context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Ppid, nil
}

func (p *Process) NameWithContext(ctx context.Context) (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}
	name := common.IntToString(k.Comm[:])

	if len(name) >= 15 {
		cmdlineSlice, err := p.CmdlineSliceWithContext(ctx)
		if err != nil {
			return "", err
		}
		if len(cmdlineSlice) > 0 {
			extendedName := filepath.Base(cmdlineSlice[0])
			if strings.HasPrefix(extendedName, p.name) {
				name = extendedName
			} else {
				name = cmdlineSlice[0]
			}
		}
	}

	return name, nil
}

func (p *Process) CwdWithContext(ctx context.Context) (string, error) {
	return "", common.ErrNotImplementedError
}

func (p *Process) ExeWithContext(ctx context.Context) (string, error) {
	mib := []int32{CTLKern, KernProc, KernProcPathname, p.Pid}
	buf, _, err := common.CallSyscall(mib)
	if err != nil {
		return "", err
	}

	return strings.Trim(string(buf), "\x00"), nil
}

func (p *Process) CmdlineWithContext(ctx context.Context) (string, error) {
	mib := []int32{CTLKern, KernProc, KernProcArgs, p.Pid}
	buf, _, err := common.CallSyscall(mib)
	if err != nil {
		return "", err
	}
	ret := strings.FieldsFunc(string(buf), func(r rune) bool {
		if r == '\u0000' {
			return true
		}
		return false
	})

	return strings.Join(ret, " "), nil
}

func (p *Process) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	mib := []int32{CTLKern, KernProc, KernProcArgs, p.Pid}
	buf, _, err := common.CallSyscall(mib)
	if err != nil {
		return nil, err
	}
	if len(buf) == 0 {
		return nil, nil
	}
	if buf[len(buf)-1] == 0 {
		buf = buf[:len(buf)-1]
	}
	parts := bytes.Split(buf, []byte{0})
	var strParts []string
	for _, p := range parts {
		strParts = append(strParts, string(p))
	}

	return strParts, nil
}

func (p *Process) createTimeWithContext(ctx context.Context) (int64, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}
	return int64(k.Start.Sec)*1000 + int64(k.Start.Usec)/1000, nil
}

func (p *Process) StatusWithContext(ctx context.Context) ([]string, error) {
	k, err := p.getKProc()
	if err != nil {
		return []string{""}, err
	}
	var s string
	switch k.Stat {
	case SIDL:
		s = Idle
	case SRUN:
		s = Running
	case SSLEEP:
		s = Sleep
	case SSTOP:
		s = Stop
	case SZOMB:
		s = Zombie
	case SWAIT:
		s = Wait
	case SLOCK:
		s = Lock
	}

	return []string{s}, nil
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

	uids := make([]int32, 0, 3)

	uids = append(uids, int32(k.Ruid), int32(k.Uid), int32(k.Svuid))

	return uids, nil
}

func (p *Process) GidsWithContext(ctx context.Context) ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	gids := make([]int32, 0, 3)
	gids = append(gids, int32(k.Rgid), int32(k.Ngroups), int32(k.Svgid))

	return gids, nil
}

func (p *Process) GroupsWithContext(ctx context.Context) ([]int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}

	groups := make([]int32, k.Ngroups)
	for i := int16(0); i < k.Ngroups; i++ {
		groups[i] = int32(k.Groups[i])
	}

	return groups, nil
}

func (p *Process) TerminalWithContext(ctx context.Context) (string, error) {
	k, err := p.getKProc()
	if err != nil {
		return "", err
	}

	ttyNr := uint64(k.Tdev)

	termmap, err := getTerminalMap()
	if err != nil {
		return "", err
	}

	return termmap[ttyNr], nil
}

func (p *Process) NiceWithContext(ctx context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}
	return int32(k.Nice), nil
}

func (p *Process) IOCountersWithContext(ctx context.Context) (*IOCountersStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	return &IOCountersStat{
		ReadCount:  uint64(k.Rusage.Inblock),
		WriteCount: uint64(k.Rusage.Oublock),
	}, nil
}

func (p *Process) NumThreadsWithContext(ctx context.Context) (int32, error) {
	k, err := p.getKProc()
	if err != nil {
		return 0, err
	}

	return k.Numthreads, nil
}

func (p *Process) TimesWithContext(ctx context.Context) (*cpu.TimesStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	return &cpu.TimesStat{
		CPU:    "cpu",
		User:   float64(k.Rusage.Utime.Sec) + float64(k.Rusage.Utime.Usec)/1000000,
		System: float64(k.Rusage.Stime.Sec) + float64(k.Rusage.Stime.Usec)/1000000,
	}, nil
}

func (p *Process) MemoryInfoWithContext(ctx context.Context) (*MemoryInfoStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	v, err := unix.Sysctl("vm.stats.vm.v_page_size")
	if err != nil {
		return nil, err
	}
	pageSize := common.LittleEndian.Uint16([]byte(v))

	return &MemoryInfoStat{
		RSS: uint64(k.Rssize) * uint64(pageSize),
		VMS: uint64(k.Size),
	}, nil
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
	return nil, common.ErrNotImplementedError
}

func (p *Process) ConnectionsMaxWithContext(ctx context.Context, max int) ([]net.ConnectionStat, error) {
	return nil, common.ErrNotImplementedError
}

func ProcessesWithContext(ctx context.Context) ([]*Process, error) {
	results := []*Process{}

	mib := []int32{CTLKern, KernProc, KernProcProc, 0}
	buf, length, err := common.CallSyscall(mib)
	if err != nil {
		return results, err
	}

	// get kinfo_proc size
	count := int(length / uint64(sizeOfKinfoProc))

	// parse buf to procs
	for i := 0; i < count; i++ {
		b := buf[i*sizeOfKinfoProc : (i+1)*sizeOfKinfoProc]
		k, err := parseKinfoProc(b)
		if err != nil {
			continue
		}
		p, err := NewProcessWithContext(ctx, int32(k.Pid))
		if err != nil {
			continue
		}

		results = append(results, p)
	}

	return results, nil
}

func (p *Process) getKProc() (*KinfoProc, error) {
	mib := []int32{CTLKern, KernProc, KernProcPID, p.Pid}

	buf, length, err := common.CallSyscall(mib)
	if err != nil {
		return nil, err
	}
	if length != sizeOfKinfoProc {
		return nil, err
	}

	k, err := parseKinfoProc(buf)
	if err != nil {
		return nil, err
	}
	return &k, nil
}
