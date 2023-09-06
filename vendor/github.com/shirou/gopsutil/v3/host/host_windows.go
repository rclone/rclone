//go:build windows
// +build windows

package host

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"

	"github.com/shirou/gopsutil/v3/internal/common"
	"github.com/shirou/gopsutil/v3/process"
	"github.com/yusufpapurcu/wmi"
	"golang.org/x/sys/windows"
)

var (
	procGetSystemTimeAsFileTime = common.Modkernel32.NewProc("GetSystemTimeAsFileTime")
	procGetTickCount32          = common.Modkernel32.NewProc("GetTickCount")
	procGetTickCount64          = common.Modkernel32.NewProc("GetTickCount64")
	procGetNativeSystemInfo     = common.Modkernel32.NewProc("GetNativeSystemInfo")
	procRtlGetVersion           = common.ModNt.NewProc("RtlGetVersion")
)

// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/ns-wdm-_osversioninfoexw
type osVersionInfoExW struct {
	dwOSVersionInfoSize uint32
	dwMajorVersion      uint32
	dwMinorVersion      uint32
	dwBuildNumber       uint32
	dwPlatformId        uint32
	szCSDVersion        [128]uint16
	wServicePackMajor   uint16
	wServicePackMinor   uint16
	wSuiteMask          uint16
	wProductType        uint8
	wReserved           uint8
}

type systemInfo struct {
	wProcessorArchitecture      uint16
	wReserved                   uint16
	dwPageSize                  uint32
	lpMinimumApplicationAddress uintptr
	lpMaximumApplicationAddress uintptr
	dwActiveProcessorMask       uintptr
	dwNumberOfProcessors        uint32
	dwProcessorType             uint32
	dwAllocationGranularity     uint32
	wProcessorLevel             uint16
	wProcessorRevision          uint16
}

type msAcpi_ThermalZoneTemperature struct {
	Active             bool
	CriticalTripPoint  uint32
	CurrentTemperature uint32
	InstanceName       string
}

func HostIDWithContext(ctx context.Context) (string, error) {
	// there has been reports of issues on 32bit using golang.org/x/sys/windows/registry, see https://github.com/shirou/gopsutil/pull/312#issuecomment-277422612
	// for rationale of using windows.RegOpenKeyEx/RegQueryValueEx instead of registry.OpenKey/GetStringValue
	var h windows.Handle
	err := windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, windows.StringToUTF16Ptr(`SOFTWARE\Microsoft\Cryptography`), 0, windows.KEY_READ|windows.KEY_WOW64_64KEY, &h)
	if err != nil {
		return "", err
	}
	defer windows.RegCloseKey(h)

	const windowsRegBufLen = 74 // len(`{`) + len(`abcdefgh-1234-456789012-123345456671` * 2) + len(`}`) // 2 == bytes/UTF16
	const uuidLen = 36

	var regBuf [windowsRegBufLen]uint16
	bufLen := uint32(windowsRegBufLen)
	var valType uint32
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`MachineGuid`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return "", err
	}

	hostID := windows.UTF16ToString(regBuf[:])
	hostIDLen := len(hostID)
	if hostIDLen != uuidLen {
		return "", fmt.Errorf("HostID incorrect: %q\n", hostID)
	}

	return strings.ToLower(hostID), nil
}

func numProcs(ctx context.Context) (uint64, error) {
	procs, err := process.PidsWithContext(ctx)
	if err != nil {
		return 0, err
	}
	return uint64(len(procs)), nil
}

func UptimeWithContext(ctx context.Context) (uint64, error) {
	up, err := uptimeMillis()
	if err != nil {
		return 0, err
	}
	return uint64((time.Duration(up) * time.Millisecond).Seconds()), nil
}

func uptimeMillis() (uint64, error) {
	procGetTickCount := procGetTickCount64
	err := procGetTickCount64.Find()
	if err != nil {
		procGetTickCount = procGetTickCount32 // handle WinXP, but keep in mind that "the time will wrap around to zero if the system is run continuously for 49.7 days." from MSDN
	}
	r1, _, lastErr := syscall.Syscall(procGetTickCount.Addr(), 0, 0, 0, 0)
	if lastErr != 0 {
		return 0, lastErr
	}
	return uint64(r1), nil
}

// cachedBootTime must be accessed via atomic.Load/StoreUint64
var cachedBootTime uint64

func BootTimeWithContext(ctx context.Context) (uint64, error) {
	t := atomic.LoadUint64(&cachedBootTime)
	if t != 0 {
		return t, nil
	}
	up, err := uptimeMillis()
	if err != nil {
		return 0, err
	}
	t = uint64((time.Duration(timeSinceMillis(up)) * time.Millisecond).Seconds())
	atomic.StoreUint64(&cachedBootTime, t)
	return t, nil
}

func PlatformInformationWithContext(ctx context.Context) (platform string, family string, version string, err error) {
	// GetVersionEx lies on Windows 8.1 and returns as Windows 8 if we don't declare compatibility in manifest
	// RtlGetVersion bypasses this lying layer and returns the true Windows version
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/nf-wdm-rtlgetversion
	// https://docs.microsoft.com/en-us/windows-hardware/drivers/ddi/content/wdm/ns-wdm-_osversioninfoexw
	var osInfo osVersionInfoExW
	osInfo.dwOSVersionInfoSize = uint32(unsafe.Sizeof(osInfo))
	ret, _, err := procRtlGetVersion.Call(uintptr(unsafe.Pointer(&osInfo)))
	if ret != 0 {
		return
	}

	// Platform
	var h windows.Handle // like HostIDWithContext(), we query the registry using the raw windows.RegOpenKeyEx/RegQueryValueEx
	err = windows.RegOpenKeyEx(windows.HKEY_LOCAL_MACHINE, windows.StringToUTF16Ptr(`SOFTWARE\Microsoft\Windows NT\CurrentVersion`), 0, windows.KEY_READ|windows.KEY_WOW64_64KEY, &h)
	if err != nil {
		return
	}
	defer windows.RegCloseKey(h)
	var bufLen uint32
	var valType uint32
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`ProductName`), nil, &valType, nil, &bufLen)
	if err != nil {
		return
	}
	regBuf := make([]uint16, bufLen/2+1)
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`ProductName`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
	if err != nil {
		return
	}
	platform = windows.UTF16ToString(regBuf[:])
	if strings.Contains(platform, "Windows 10") { // check build number to determine whether it's actually Windows 11
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CurrentBuildNumber`), nil, &valType, nil, &bufLen)
		if err == nil {
			regBuf = make([]uint16, bufLen/2+1)
			err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CurrentBuildNumber`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
			if err == nil {
				buildNumberStr := windows.UTF16ToString(regBuf[:])
				if buildNumber, err := strconv.Atoi(buildNumberStr); err == nil && buildNumber >= 22000 {
					platform = strings.Replace(platform, "Windows 10", "Windows 11", 1)
				}
			}
		}
	}
	if !strings.HasPrefix(platform, "Microsoft") {
		platform = "Microsoft " + platform
	}
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CSDVersion`), nil, &valType, nil, &bufLen) // append Service Pack number, only on success
	if err == nil {                                                                                       // don't return an error if only the Service Pack retrieval fails
		regBuf = make([]uint16, bufLen/2+1)
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`CSDVersion`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
		if err == nil {
			platform += " " + windows.UTF16ToString(regBuf[:])
		}
	}

	var UBR uint32 // Update Build Revision
	err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`UBR`), nil, &valType, nil, &bufLen)
	if err == nil {
		regBuf := make([]byte, 4)
		err = windows.RegQueryValueEx(h, windows.StringToUTF16Ptr(`UBR`), nil, &valType, (*byte)(unsafe.Pointer(&regBuf[0])), &bufLen)
		copy((*[4]byte)(unsafe.Pointer(&UBR))[:], regBuf)
	}

	// PlatformFamily
	switch osInfo.wProductType {
	case 1:
		family = "Standalone Workstation"
	case 2:
		family = "Server (Domain Controller)"
	case 3:
		family = "Server"
	}

	// Platform Version
	version = fmt.Sprintf("%d.%d.%d.%d Build %d.%d",
		osInfo.dwMajorVersion, osInfo.dwMinorVersion, osInfo.dwBuildNumber, UBR,
		osInfo.dwBuildNumber, UBR)

	return platform, family, version, nil
}

func UsersWithContext(ctx context.Context) ([]UserStat, error) {
	var ret []UserStat

	return ret, common.ErrNotImplementedError
}

func SensorsTemperaturesWithContext(ctx context.Context) ([]TemperatureStat, error) {
	var ret []TemperatureStat
	var dst []msAcpi_ThermalZoneTemperature
	q := wmi.CreateQuery(&dst, "")
	if err := common.WMIQueryWithContext(ctx, q, &dst, nil, "root/wmi"); err != nil {
		return ret, err
	}

	for _, v := range dst {
		ts := TemperatureStat{
			SensorKey:   v.InstanceName,
			Temperature: kelvinToCelsius(v.CurrentTemperature, 2),
		}
		ret = append(ret, ts)
	}

	return ret, nil
}

func kelvinToCelsius(temp uint32, n int) float64 {
	// wmi return temperature Kelvin * 10, so need to divide the result by 10,
	// and then minus 273.15 to get °Celsius.
	t := float64(temp/10) - 273.15
	n10 := math.Pow10(n)
	return math.Trunc((t+0.5/n10)*n10) / n10
}

func VirtualizationWithContext(ctx context.Context) (string, string, error) {
	return "", "", common.ErrNotImplementedError
}

func KernelVersionWithContext(ctx context.Context) (string, error) {
	_, _, version, err := PlatformInformationWithContext(ctx)
	return version, err
}

func KernelArch() (string, error) {
	var systemInfo systemInfo
	procGetNativeSystemInfo.Call(uintptr(unsafe.Pointer(&systemInfo)))

	const (
		PROCESSOR_ARCHITECTURE_INTEL = 0
		PROCESSOR_ARCHITECTURE_ARM   = 5
		PROCESSOR_ARCHITECTURE_ARM64 = 12
		PROCESSOR_ARCHITECTURE_IA64  = 6
		PROCESSOR_ARCHITECTURE_AMD64 = 9
	)
	switch systemInfo.wProcessorArchitecture {
	case PROCESSOR_ARCHITECTURE_INTEL:
		if systemInfo.wProcessorLevel < 3 {
			return "i386", nil
		}
		if systemInfo.wProcessorLevel > 6 {
			return "i686", nil
		}
		return fmt.Sprintf("i%d86", systemInfo.wProcessorLevel), nil
	case PROCESSOR_ARCHITECTURE_ARM:
		return "arm", nil
	case PROCESSOR_ARCHITECTURE_ARM64:
		return "aarch64", nil
	case PROCESSOR_ARCHITECTURE_IA64:
		return "ia64", nil
	case PROCESSOR_ARCHITECTURE_AMD64:
		return "x86_64", nil
	}
	return "", nil
}
