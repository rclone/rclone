// Copyright (c) 2015 Klaus Post, released under MIT License. See LICENSE file.

//+build 386,!gccgo amd64,!gccgo,!noasm,!appengine

package cpuid

func asmCpuid(op uint32) (eax, ebx, ecx, edx uint32)
func asmCpuidex(op, op2 uint32) (eax, ebx, ecx, edx uint32)
func asmXgetbv(index uint32) (eax, edx uint32)
func asmRdtscpAsm() (eax, ebx, ecx, edx uint32)

func initCPU() {
	cpuid = asmCpuid
	cpuidex = asmCpuidex
	xgetbv = asmXgetbv
	rdtscpAsm = asmRdtscpAsm
}

func addInfo(c *CPUInfo) {
	c.maxFunc = maxFunctionID()
	c.maxExFunc = maxExtendedFunction()
	c.BrandName = brandName()
	c.CacheLine = cacheLine()
	c.Family, c.Model = familyModel()
	c.Features = support()
	c.SGX = hasSGX(c.Features&SGX != 0, c.Features&SGXLC != 0)
	c.ThreadsPerCore = threadsPerCore()
	c.LogicalCores = logicalCores()
	c.PhysicalCores = physicalCores()
	c.VendorID, c.VendorString = vendorID()
	c.Hz = hertz(c.BrandName)
	c.cacheSize()
}
