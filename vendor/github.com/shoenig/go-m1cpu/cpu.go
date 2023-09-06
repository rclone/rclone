//go:build darwin && arm64 && cgo

package m1cpu

// #cgo LDFLAGS: -framework CoreFoundation -framework IOKit
// #include <AvailabilityMacros.h>
// #include <CoreFoundation/CoreFoundation.h>
// #include <IOKit/IOKitLib.h>
// #include <sys/sysctl.h>
//
// #if !defined(MAC_OS_VERSION_12_0) || MAC_OS_X_VERSION_MIN_REQUIRED < MAC_OS_VERSION_12_0
// #define kIOMainPortDefault kIOMasterPortDefault
// #endif
//
// #define HzToGHz(hz) ((hz) / 1000000000.0)
//
// UInt64 global_pCoreHz;
// UInt64 global_eCoreHz;
// int global_pCoreCount;
// int global_eCoreCount;
// int global_pCoreL1InstCacheSize;
// int global_eCoreL1InstCacheSize;
// int global_pCoreL1DataCacheSize;
// int global_eCoreL1DataCacheSize;
// int global_pCoreL2CacheSize;
// int global_eCoreL2CacheSize;
// char global_brand[32];
//
// UInt64 getFrequency(CFTypeRef typeRef) {
// CFDataRef cfData = typeRef;
//
// CFIndex size = CFDataGetLength(cfData);
// UInt8 buf[size];
// CFDataGetBytes(cfData, CFRangeMake(0, size), buf);
//
// UInt8 b1 = buf[size-5];
// UInt8 b2 = buf[size-6];
// UInt8 b3 = buf[size-7];
// UInt8 b4 = buf[size-8];
//
// UInt64 pCoreHz = 0x00000000FFFFFFFF & ((b1<<24) | (b2 << 16) | (b3 << 8) | (b4));
// return pCoreHz;
// }
//
// int sysctl_int(const char * name) {
//  int value = -1;
//  size_t size = 8;
//  sysctlbyname(name, &value, &size, NULL, 0);
//  return value;
// }
//
// void sysctl_string(const char * name, char * dest) {
//   size_t size = 32;
//   sysctlbyname(name, dest, &size, NULL, 0);
// }
//
// void initialize() {
//   global_pCoreCount = sysctl_int("hw.perflevel0.physicalcpu");
//   global_eCoreCount = sysctl_int("hw.perflevel1.physicalcpu");
//   global_pCoreL1InstCacheSize = sysctl_int("hw.perflevel0.l1icachesize");
//   global_eCoreL1InstCacheSize = sysctl_int("hw.perflevel1.l1icachesize");
//   global_pCoreL1DataCacheSize = sysctl_int("hw.perflevel0.l1dcachesize");
//   global_eCoreL1DataCacheSize = sysctl_int("hw.perflevel1.l1dcachesize");
//   global_pCoreL2CacheSize = sysctl_int("hw.perflevel0.l2cachesize");
//   global_eCoreL2CacheSize = sysctl_int("hw.perflevel1.l2cachesize");
//   sysctl_string("machdep.cpu.brand_string", global_brand);
//
//   CFMutableDictionaryRef matching = IOServiceMatching("AppleARMIODevice");
//   io_iterator_t  iter;
//   IOServiceGetMatchingServices(kIOMainPortDefault, matching, &iter);
//
//   const size_t bufsize = 512;
//   io_object_t obj;
//   while ((obj = IOIteratorNext(iter))) {
//     char class[bufsize];
//     IOObjectGetClass(obj, class);
//     char name[bufsize];
//     IORegistryEntryGetName(obj, name);
//
//     if (strncmp(name, "pmgr", bufsize) == 0) {
//       CFTypeRef pCoreRef = IORegistryEntryCreateCFProperty(obj, CFSTR("voltage-states5-sram"), kCFAllocatorDefault, 0);
//       CFTypeRef eCoreRef = IORegistryEntryCreateCFProperty(obj, CFSTR("voltage-states1-sram"), kCFAllocatorDefault, 0);
//
//       long long pCoreHz = getFrequency(pCoreRef);
//       long long eCoreHz = getFrequency(eCoreRef);
//
//       global_pCoreHz = pCoreHz;
//       global_eCoreHz = eCoreHz;
//       return;
//     }
//   }
// }
//
// UInt64 eCoreHz() {
//   return global_eCoreHz;
// }
//
// UInt64 pCoreHz() {
//   return global_pCoreHz;
// }
//
// Float64 eCoreGHz() {
//   return HzToGHz(global_eCoreHz);
// }
//
// Float64 pCoreGHz() {
//   return HzToGHz(global_pCoreHz);
// }
//
// int pCoreCount() {
//   return global_pCoreCount;
// }
//
// int eCoreCount() {
//   return global_eCoreCount;
// }
//
// int pCoreL1InstCacheSize() {
//   return global_pCoreL1InstCacheSize;
// }
//
// int pCoreL1DataCacheSize() {
//   return global_pCoreL1DataCacheSize;
// }
//
// int pCoreL2CacheSize() {
//   return global_pCoreL2CacheSize;
// }
//
// int eCoreL1InstCacheSize() {
//   return global_eCoreL1InstCacheSize;
// }
//
// int eCoreL1DataCacheSize() {
//   return global_eCoreL1DataCacheSize;
// }
//
// int eCoreL2CacheSize() {
//   return global_eCoreL2CacheSize;
// }
//
// char * modelName() {
//   return global_brand;
// }
import "C"

func init() {
	C.initialize()
}

// IsAppleSilicon returns true on this platform.
func IsAppleSilicon() bool {
	return true
}

// PCoreHZ returns the max frequency in Hertz of the P-Core of an Apple Silicon CPU.
func PCoreHz() uint64 {
	return uint64(C.pCoreHz())
}

// ECoreHZ returns the max frequency in Hertz of the E-Core of an Apple Silicon CPU.
func ECoreHz() uint64 {
	return uint64(C.eCoreHz())
}

// PCoreGHz returns the max frequency in Gigahertz of the P-Core of an Apple Silicon CPU.
func PCoreGHz() float64 {
	return float64(C.pCoreGHz())
}

// ECoreGHz returns the max frequency in Gigahertz of the E-Core of an Apple Silicon CPU.
func ECoreGHz() float64 {
	return float64(C.eCoreGHz())
}

// PCoreCount returns the number of physical P (performance) cores.
func PCoreCount() int {
	return int(C.pCoreCount())
}

// ECoreCount returns the number of physical E (efficiency) cores.
func ECoreCount() int {
	return int(C.eCoreCount())
}

// PCoreCacheSize returns the sizes of the P (performance) core cache sizes
// in the order of
//
// - L1 instruction cache
// - L1 data cache
// - L2 cache
func PCoreCache() (int, int, int) {
	return int(C.pCoreL1InstCacheSize()),
		int(C.pCoreL1DataCacheSize()),
		int(C.pCoreL2CacheSize())
}

// ECoreCacheSize returns the sizes of the E (efficiency) core cache sizes
// in the order of
//
// - L1 instruction cache
// - L1 data cache
// - L2 cache
func ECoreCache() (int, int, int) {
	return int(C.eCoreL1InstCacheSize()),
		int(C.eCoreL1DataCacheSize()),
		int(C.eCoreL2CacheSize())
}

// ModelName returns the model name of the CPU.
func ModelName() string {
	return C.GoString(C.modelName())
}
