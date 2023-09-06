//go:build openbsd
// +build openbsd

package process

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"unsafe"

	cpu "github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/internal/common"
	mem "github.com/shirou/gopsutil/v3/mem"
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
	return "", common.ErrNotImplementedError
}

func (p *Process) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	mib := []int32{CTLKern, KernProcArgs, p.Pid, KernProcArgv}
	buf, _, err := common.CallSyscall(mib)
	if err != nil {
		return nil, err
	}

	/* From man sysctl(2):
	The buffer pointed to by oldp is filled with an array of char
	pointers followed by the strings themselves. The last char
	pointer is a NULL pointer. */
	var strParts []string
	r := bytes.NewReader(buf)
	baseAddr := uintptr(unsafe.Pointer(&buf[0]))
	for {
		argvp, err := readPtr(r)
		if err != nil {
			return nil, err
		}
		if argvp == 0 { // check for a NULL pointer
			break
		}
		offset := argvp - baseAddr
		length := uintptr(bytes.IndexByte(buf[offset:], 0))
		str := string(buf[offset : offset+length])
		strParts = append(strParts, str)
	}

	return strParts, nil
}

// readPtr reads a pointer data from a given reader. WARNING: only little
// endian architectures are supported.
func readPtr(r io.Reader) (uintptr, error) {
	switch sizeofPtr {
	case 4:
		var p uint32
		if err := binary.Read(r, binary.LittleEndian, &p); err != nil {
			return 0, err
		}
		return uintptr(p), nil
	case 8:
		var p uint64
		if err := binary.Read(r, binary.LittleEndian, &p); err != nil {
			return 0, err
		}
		return uintptr(p), nil
	default:
		return 0, fmt.Errorf("unsupported pointer size")
	}
}

func (p *Process) CmdlineWithContext(ctx context.Context) (string, error) {
	argv, err := p.CmdlineSliceWithContext(ctx)
	if err != nil {
		return "", err
	}
	return strings.Join(argv, " "), nil
}

func (p *Process) createTimeWithContext(ctx context.Context) (int64, error) {
	return 0, common.ErrNotImplementedError
}

func (p *Process) StatusWithContext(ctx context.Context) ([]string, error) {
	k, err := p.getKProc()
	if err != nil {
		return []string{""}, err
	}
	var s string
	switch k.Stat {
	case SIDL:
	case SRUN:
	case SONPROC:
		s = Running
	case SSLEEP:
		s = Sleep
	case SSTOP:
		s = Stop
	case SDEAD:
		s = Zombie
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
		ReadCount:  uint64(k.Uru_inblock),
		WriteCount: uint64(k.Uru_oublock),
	}, nil
}

func (p *Process) NumThreadsWithContext(ctx context.Context) (int32, error) {
	/* not supported, just return 1 */
	return 1, nil
}

func (p *Process) TimesWithContext(ctx context.Context) (*cpu.TimesStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	return &cpu.TimesStat{
		CPU:    "cpu",
		User:   float64(k.Uutime_sec) + float64(k.Uutime_usec)/1000000,
		System: float64(k.Ustime_sec) + float64(k.Ustime_usec)/1000000,
	}, nil
}

func (p *Process) MemoryInfoWithContext(ctx context.Context) (*MemoryInfoStat, error) {
	k, err := p.getKProc()
	if err != nil {
		return nil, err
	}
	pageSize, err := mem.GetPageSizeWithContext(ctx)
	if err != nil {
		return nil, err
	}

	return &MemoryInfoStat{
		RSS: uint64(k.Vm_rssize) * pageSize,
		VMS: uint64(k.Vm_tsize) + uint64(k.Vm_dsize) +
			uint64(k.Vm_ssize),
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

	buf, length, err := callKernProcSyscall(KernProcAll, 0)
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
	buf, length, err := callKernProcSyscall(KernProcPID, p.Pid)
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

func callKernProcSyscall(op int32, arg int32) ([]byte, uint64, error) {
	mib := []int32{CTLKern, KernProc, op, arg, sizeOfKinfoProc, 0}
	mibptr := unsafe.Pointer(&mib[0])
	miblen := uint64(len(mib))
	length := uint64(0)
	_, _, err := unix.Syscall6(
		unix.SYS___SYSCTL,
		uintptr(mibptr),
		uintptr(miblen),
		0,
		uintptr(unsafe.Pointer(&length)),
		0,
		0)
	if err != 0 {
		return nil, length, err
	}

	count := int32(length / uint64(sizeOfKinfoProc))
	mib = []int32{CTLKern, KernProc, op, arg, sizeOfKinfoProc, count}
	mibptr = unsafe.Pointer(&mib[0])
	miblen = uint64(len(mib))
	// get proc info itself
	buf := make([]byte, length)
	_, _, err = unix.Syscall6(
		unix.SYS___SYSCTL,
		uintptr(mibptr),
		uintptr(miblen),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&length)),
		0,
		0)
	if err != 0 {
		return buf, length, err
	}

	return buf, length, nil
}
