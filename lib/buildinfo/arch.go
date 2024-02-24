package buildinfo

import (
	"runtime"

	"golang.org/x/sys/cpu"
)

// GetSupportedGOARM returns the ARM compatibility level of the current CPU.
//
// Returns the integer value that can be set for the GOARM variable to
// build with this level as target, a value which normally corresponds to the
// ARM architecture version number, although it is the floating point hardware
// support which is the decicive factor.
//
// Only relevant for 32-bit ARM architectures, where GOARCH=arm, which means
// ARMv7 and lower (ARMv8 is GOARCH=arm64 and GOARM is not considered).
// Highest possible value is therefore 7, while other possible values are
// 6 (for ARMv6) and 5 (for ARMv5, which is the lowest currently supported
// in go. Returns value 0 for anything else.
//
// See also:
//
//	https://go.dev/src/runtime/os_linux_arm.go
//	https://github.com/golang/go/wiki/GoArm
func GetSupportedGOARM() int {
	if runtime.GOARCH == "arm" && cpu.Initialized {
		// This CPU is an ARM (32-bit), and cpu.Initialized true means its
		// features could be retrieved on current GOOS so that we can check
		// for floating point hardware support.
		if cpu.ARM.HasVFPv3 {
			// This CPU has VFPv3 floating point hardware, which means it can
			// run programs built with any GOARM value, 7 and lower.
			return 7
		} else if cpu.ARM.HasVFP {
			// This CPU has VFP floating point hardware, but not VFPv3, which
			// means it can run programs built with GOARM value 6 and lower,
			// but not 7.
			return 6
		}
		// This CPU has no VFP floating point hardware, which means it can
		// only run programs built with GOARM value 5, which is minimum supported.
		// Note that the CPU can still in reality be based on e.g. ARMv7
		// architecture, but simply lack hardfloat support.
		return 5
	}
	return 0
}

// GetArch tells the rclone executable's architecture target.
func GetArch() string {
	// Get the running program's architecture target.
	arch := runtime.GOARCH

	// For ARM architectures there are several variants, with different
	// inconsistent and ambiguous naming.
	//
	// The most interesting thing here is which compatibility level of go is
	// used, as controlled by GOARM build variable. We cannot in runtime get
	// the actual value of GOARM used for building this program, but we can
	// check the value supported by the current CPU by calling GetSupportedGOARM.
	// This means we return information about the compatibility level (GOARM
	// value) supported, when the current rclone executable may in reality be
	// built with a lower level.
	//
	// Note that the kernel architecture, as returned by "uname -m", is not
	// considered or included in results here, but it is included in the output
	// from function GetOSVersion. It can have values such as armv6l, armv7l,
	// armv8l, arm64 and aarch64, which may give relevant information. But it
	// can also simply have value "arm", or it can have value "armv7l" for a
	// processor based on ARMv7 but without floating point hardware - which
	// means it in go needs to be built in ARMv5 compatibility mode (GOARM=5).
	if arch == "arm64" {
		// 64-bit ARM architecture, known as AArch64, was introduced with ARMv8.
		// In go this architecture is a specific one, separate from other ARMs.
		arch += " (ARMv8 compatible)"
	} else if arch == "arm" {
		// 32-bit ARM architecture, which is ARMv7 and lower.
		// In go there are different compatibility levels represented by ARM
		// architecture version number (like 5, 6 or 7).
		switch GetSupportedGOARM() {
		case 7:
			arch += " (ARMv7 compatible)"
		case 6:
			arch += " (ARMv6 compatible)"
		case 5:
			arch += " (ARMv5 compatible, no hardfloat)"
		}
	}
	return arch
}
