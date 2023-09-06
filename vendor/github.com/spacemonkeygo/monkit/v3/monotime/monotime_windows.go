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
	qpcBase      = queryPerformanceCounter()
)

func elapsed() time.Duration {
	elapsed := queryPerformanceCounter() - qpcBase
	return time.Duration(elapsed) * time.Second / (time.Duration(qpcFrequency) * time.Nanosecond)
}

func queryPerformanceCounter() int64 {
	var count int64
	syscall.Syscall(queryPerformanceCounterProc.Addr(), 1, uintptr(unsafe.Pointer(&count)), 0, 0)
	return count
}

func queryPerformanceFrequency() int64 {
	var freq int64
	syscall.Syscall(queryPerformanceFrequencyProc.Addr(), 1, uintptr(unsafe.Pointer(&freq)), 0, 0)
	return freq
}
