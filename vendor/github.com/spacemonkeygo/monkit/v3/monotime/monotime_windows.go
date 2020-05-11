package monotime

import (
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32                   = syscall.NewLazyDLL("kernel32.dll")
	queryPerformanceFrequencyProc = modkernel32.NewProc("QueryPerformanceFrequency")
	queryPerformanceCounterProc   = modkernel32.NewProc("QueryPerformanceCounter")

	qpcFrequency = queryPerformanceFrequency()
)

func elapsed() time.Duration {
	var elapsed int64
	syscall.Syscall(queryPerformanceCounterProc.Addr(), 1, uintptr(unsafe.Pointer(&elapsed)), 0, 0)
	return time.Duration(elapsed) * time.Second / (time.Duration(qpcFrequency) * time.Nanosecond)
}

func queryPerformanceFrequency() int64 {
	var freq int64
	syscall.Syscall(queryPerformanceFrequencyProc.Addr(), 1, uintptr(unsafe.Pointer(&freq)), 0, 0)
	return freq
}
