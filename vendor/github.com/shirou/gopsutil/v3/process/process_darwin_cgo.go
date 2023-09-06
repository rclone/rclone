//go:build darwin && cgo
// +build darwin,cgo

package process

// #include <stdlib.h>
// #include <libproc.h>
// #include <string.h>
// #include <sys/errno.h>
// #include <sys/proc_info.h>
// #include <sys/sysctl.h>
// #include <mach/mach_time.h>
import "C"

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/shirou/gopsutil/v3/cpu"
)

var (
	argMax                 int
	timescaleToNanoSeconds float64
)

func init() {
	argMax = getArgMax()
	timescaleToNanoSeconds = getTimeScaleToNanoSeconds()
}

func getArgMax() int {
	var (
		mib    = [...]C.int{C.CTL_KERN, C.KERN_ARGMAX}
		argmax C.int
		size   C.size_t = C.ulong(unsafe.Sizeof(argmax))
	)
	retval := C.sysctl(&mib[0], 2, unsafe.Pointer(&argmax), &size, C.NULL, 0)
	if retval == 0 {
		return int(argmax)
	}
	return 0
}

func getTimeScaleToNanoSeconds() float64 {
	var timeBaseInfo C.struct_mach_timebase_info

	C.mach_timebase_info(&timeBaseInfo)

	return float64(timeBaseInfo.numer) / float64(timeBaseInfo.denom)
}

func (p *Process) ExeWithContext(ctx context.Context) (string, error) {
	var c C.char // need a var for unsafe.Sizeof need a var
	const bufsize = C.PROC_PIDPATHINFO_MAXSIZE * unsafe.Sizeof(c)
	buffer := (*C.char)(C.malloc(C.size_t(bufsize)))
	defer C.free(unsafe.Pointer(buffer))

	ret, err := C.proc_pidpath(C.int(p.Pid), unsafe.Pointer(buffer), C.uint32_t(bufsize))
	if err != nil {
		return "", err
	}
	if ret <= 0 {
		return "", fmt.Errorf("unknown error: proc_pidpath returned %d", ret)
	}

	return C.GoString(buffer), nil
}

// CwdWithContext retrieves the Current Working Directory for the given process.
// It uses the proc_pidinfo from libproc and will only work for processes the
// EUID can access.  Otherwise "operation not permitted" will be returned as the
// error.
// Note: This might also work for other *BSD OSs.
func (p *Process) CwdWithContext(ctx context.Context) (string, error) {
	const vpiSize = C.sizeof_struct_proc_vnodepathinfo
	vpi := (*C.struct_proc_vnodepathinfo)(C.malloc(vpiSize))
	defer C.free(unsafe.Pointer(vpi))
	ret, err := C.proc_pidinfo(C.int(p.Pid), C.PROC_PIDVNODEPATHINFO, 0, unsafe.Pointer(vpi), vpiSize)
	if err != nil {
		// fmt.Printf("ret: %d %T\n", ret, err)
		if err == syscall.EPERM {
			return "", ErrorNotPermitted
		}
		return "", err
	}
	if ret <= 0 {
		return "", fmt.Errorf("unknown error: proc_pidinfo returned %d", ret)
	}
	if ret != C.sizeof_struct_proc_vnodepathinfo {
		return "", fmt.Errorf("too few bytes; expected %d, got %d", vpiSize, ret)
	}
	return C.GoString(&vpi.pvi_cdir.vip_path[0]), err
}

func procArgs(pid int32) ([]byte, int, error) {
	var (
		mib             = [...]C.int{C.CTL_KERN, C.KERN_PROCARGS2, C.int(pid)}
		size   C.size_t = C.ulong(argMax)
		nargs  C.int
		result []byte
	)
	procargs := (*C.char)(C.malloc(C.ulong(argMax)))
	defer C.free(unsafe.Pointer(procargs))
	retval, err := C.sysctl(&mib[0], 3, unsafe.Pointer(procargs), &size, C.NULL, 0)
	if retval == 0 {
		C.memcpy(unsafe.Pointer(&nargs), unsafe.Pointer(procargs), C.sizeof_int)
		result = C.GoBytes(unsafe.Pointer(procargs), C.int(size))
		// fmt.Printf("size: %d %d\n%s\n", size, nargs, hex.Dump(result))
		return result, int(nargs), nil
	}
	return nil, 0, err
}

func (p *Process) CmdlineSliceWithContext(ctx context.Context) ([]string, error) {
	return p.cmdlineSliceWithContext(ctx, true)
}

func (p *Process) cmdlineSliceWithContext(ctx context.Context, fallback bool) ([]string, error) {
	pargs, nargs, err := procArgs(p.Pid)
	if err != nil {
		return nil, err
	}
	// The first bytes hold the nargs int, skip it.
	args := bytes.Split((pargs)[C.sizeof_int:], []byte{0})
	var argStr string
	// The first element is the actual binary/command path.
	// command := args[0]
	var argSlice []string
	// var envSlice []string
	// All other, non-zero elements are arguments. The first "nargs" elements
	// are the arguments. Everything else in the slice is then the environment
	// of the process.
	for _, arg := range args[1:] {
		argStr = string(arg[:])
		if len(argStr) > 0 {
			if nargs > 0 {
				argSlice = append(argSlice, argStr)
				nargs--
				continue
			}
			break
			// envSlice = append(envSlice, argStr)
		}
	}
	return argSlice, err
}

// cmdNameWithContext returns the command name (including spaces) without any arguments
func (p *Process) cmdNameWithContext(ctx context.Context) (string, error) {
	r, err := p.cmdlineSliceWithContext(ctx, false)
	if err != nil {
		return "", err
	}

	if len(r) == 0 {
		return "", nil
	}

	return r[0], err
}

func (p *Process) CmdlineWithContext(ctx context.Context) (string, error) {
	r, err := p.CmdlineSliceWithContext(ctx)
	if err != nil {
		return "", err
	}
	return strings.Join(r, " "), err
}

func (p *Process) NumThreadsWithContext(ctx context.Context) (int32, error) {
	const tiSize = C.sizeof_struct_proc_taskinfo
	ti := (*C.struct_proc_taskinfo)(C.malloc(tiSize))
	defer C.free(unsafe.Pointer(ti))

	_, err := C.proc_pidinfo(C.int(p.Pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(ti), tiSize)
	if err != nil {
		return 0, err
	}

	return int32(ti.pti_threadnum), nil
}

func (p *Process) TimesWithContext(ctx context.Context) (*cpu.TimesStat, error) {
	const tiSize = C.sizeof_struct_proc_taskinfo
	ti := (*C.struct_proc_taskinfo)(C.malloc(tiSize))
	defer C.free(unsafe.Pointer(ti))

	_, err := C.proc_pidinfo(C.int(p.Pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(ti), tiSize)
	if err != nil {
		return nil, err
	}

	ret := &cpu.TimesStat{
		CPU:    "cpu",
		User:   float64(ti.pti_total_user) * timescaleToNanoSeconds / 1e9,
		System: float64(ti.pti_total_system) * timescaleToNanoSeconds / 1e9,
	}
	return ret, nil
}

func (p *Process) MemoryInfoWithContext(ctx context.Context) (*MemoryInfoStat, error) {
	const tiSize = C.sizeof_struct_proc_taskinfo
	ti := (*C.struct_proc_taskinfo)(C.malloc(tiSize))
	defer C.free(unsafe.Pointer(ti))

	_, err := C.proc_pidinfo(C.int(p.Pid), C.PROC_PIDTASKINFO, 0, unsafe.Pointer(ti), tiSize)
	if err != nil {
		return nil, err
	}

	ret := &MemoryInfoStat{
		RSS:  uint64(ti.pti_resident_size),
		VMS:  uint64(ti.pti_virtual_size),
		Swap: uint64(ti.pti_pageins),
	}
	return ret, nil
}
