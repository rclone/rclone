// Copyright (c) 2015 Klaus Post, released under MIT License. See LICENSE file.

//+build arm64,!gccgo,!noasm,!appengine

package cpuid

func getMidr() (midr uint64)
func getProcFeatures() (procFeatures uint64)
func getInstAttributes() (instAttrReg0, instAttrReg1 uint64)

func initCPU() {
	cpuid = func(uint32) (a, b, c, d uint32) { return 0, 0, 0, 0 }
	cpuidex = func(x, y uint32) (a, b, c, d uint32) { return 0, 0, 0, 0 }
	xgetbv = func(uint32) (a, b uint32) { return 0, 0 }
	rdtscpAsm = func() (a, b, c, d uint32) { return 0, 0, 0, 0 }
}

func addInfo(c *CPUInfo) {
	// 	midr := getMidr()

	// MIDR_EL1 - Main ID Register
	//  x--------------------------------------------------x
	//  | Name                         |  bits   | visible |
	//  |--------------------------------------------------|
	//  | Implementer                  | [31-24] |    y    |
	//  |--------------------------------------------------|
	//  | Variant                      | [23-20] |    y    |
	//  |--------------------------------------------------|
	//  | Architecture                 | [19-16] |    y    |
	//  |--------------------------------------------------|
	//  | PartNum                      | [15-4]  |    y    |
	//  |--------------------------------------------------|
	//  | Revision                     | [3-0]   |    y    |
	//  x--------------------------------------------------x

	// 	fmt.Printf(" implementer:  0x%02x\n", (midr>>24)&0xff)
	// 	fmt.Printf("     variant:   0x%01x\n", (midr>>20)&0xf)
	// 	fmt.Printf("architecture:   0x%01x\n", (midr>>16)&0xf)
	// 	fmt.Printf("    part num: 0x%03x\n", (midr>>4)&0xfff)
	// 	fmt.Printf("    revision:   0x%01x\n", (midr>>0)&0xf)

	procFeatures := getProcFeatures()

	// ID_AA64PFR0_EL1 - Processor Feature Register 0
	// x--------------------------------------------------x
	// | Name                         |  bits   | visible |
	// |--------------------------------------------------|
	// | DIT                          | [51-48] |    y    |
	// |--------------------------------------------------|
	// | SVE                          | [35-32] |    y    |
	// |--------------------------------------------------|
	// | GIC                          | [27-24] |    n    |
	// |--------------------------------------------------|
	// | AdvSIMD                      | [23-20] |    y    |
	// |--------------------------------------------------|
	// | FP                           | [19-16] |    y    |
	// |--------------------------------------------------|
	// | EL3                          | [15-12] |    n    |
	// |--------------------------------------------------|
	// | EL2                          | [11-8]  |    n    |
	// |--------------------------------------------------|
	// | EL1                          | [7-4]   |    n    |
	// |--------------------------------------------------|
	// | EL0                          | [3-0]   |    n    |
	// x--------------------------------------------------x

	var f ArmFlags
	// if procFeatures&(0xf<<48) != 0 {
	// 	fmt.Println("DIT")
	// }
	if procFeatures&(0xf<<32) != 0 {
		f |= SVE
	}
	if procFeatures&(0xf<<20) != 15<<20 {
		f |= ASIMD
		if procFeatures&(0xf<<20) == 1<<20 {
			// https://developer.arm.com/docs/ddi0595/b/aarch64-system-registers/id_aa64pfr0_el1
			// 0b0001 --> As for 0b0000, and also includes support for half-precision floating-point arithmetic.
			f |= FPHP
			f |= ASIMDHP
		}
	}
	if procFeatures&(0xf<<16) != 0 {
		f |= FP
	}

	instAttrReg0, instAttrReg1 := getInstAttributes()

	// https://developer.arm.com/docs/ddi0595/b/aarch64-system-registers/id_aa64isar0_el1
	//
	// ID_AA64ISAR0_EL1 - Instruction Set Attribute Register 0
	// x--------------------------------------------------x
	// | Name                         |  bits   | visible |
	// |--------------------------------------------------|
	// | TS                           | [55-52] |    y    |
	// |--------------------------------------------------|
	// | FHM                          | [51-48] |    y    |
	// |--------------------------------------------------|
	// | DP                           | [47-44] |    y    |
	// |--------------------------------------------------|
	// | SM4                          | [43-40] |    y    |
	// |--------------------------------------------------|
	// | SM3                          | [39-36] |    y    |
	// |--------------------------------------------------|
	// | SHA3                         | [35-32] |    y    |
	// |--------------------------------------------------|
	// | RDM                          | [31-28] |    y    |
	// |--------------------------------------------------|
	// | ATOMICS                      | [23-20] |    y    |
	// |--------------------------------------------------|
	// | CRC32                        | [19-16] |    y    |
	// |--------------------------------------------------|
	// | SHA2                         | [15-12] |    y    |
	// |--------------------------------------------------|
	// | SHA1                         | [11-8]  |    y    |
	// |--------------------------------------------------|
	// | AES                          | [7-4]   |    y    |
	// x--------------------------------------------------x

	// if instAttrReg0&(0xf<<52) != 0 {
	// 	fmt.Println("TS")
	// }
	// if instAttrReg0&(0xf<<48) != 0 {
	// 	fmt.Println("FHM")
	// }
	if instAttrReg0&(0xf<<44) != 0 {
		f |= ASIMDDP
	}
	if instAttrReg0&(0xf<<40) != 0 {
		f |= SM4
	}
	if instAttrReg0&(0xf<<36) != 0 {
		f |= SM3
	}
	if instAttrReg0&(0xf<<32) != 0 {
		f |= SHA3
	}
	if instAttrReg0&(0xf<<28) != 0 {
		f |= ASIMDRDM
	}
	if instAttrReg0&(0xf<<20) != 0 {
		f |= ATOMICS
	}
	if instAttrReg0&(0xf<<16) != 0 {
		f |= CRC32
	}
	if instAttrReg0&(0xf<<12) != 0 {
		f |= SHA2
	}
	if instAttrReg0&(0xf<<12) == 2<<12 {
		// https://developer.arm.com/docs/ddi0595/b/aarch64-system-registers/id_aa64isar0_el1
		// 0b0010 --> As 0b0001, plus SHA512H, SHA512H2, SHA512SU0, and SHA512SU1 instructions implemented.
		f |= SHA512
	}
	if instAttrReg0&(0xf<<8) != 0 {
		f |= SHA1
	}
	if instAttrReg0&(0xf<<4) != 0 {
		f |= AES
	}
	if instAttrReg0&(0xf<<4) == 2<<4 {
		// https://developer.arm.com/docs/ddi0595/b/aarch64-system-registers/id_aa64isar0_el1
		// 0b0010 --> As for 0b0001, plus PMULL/PMULL2 instructions operating on 64-bit data quantities.
		f |= PMULL
	}

	// https://developer.arm.com/docs/ddi0595/b/aarch64-system-registers/id_aa64isar1_el1
	//
	// ID_AA64ISAR1_EL1 - Instruction set attribute register 1
	// x--------------------------------------------------x
	// | Name                         |  bits   | visible |
	// |--------------------------------------------------|
	// | GPI                          | [31-28] |    y    |
	// |--------------------------------------------------|
	// | GPA                          | [27-24] |    y    |
	// |--------------------------------------------------|
	// | LRCPC                        | [23-20] |    y    |
	// |--------------------------------------------------|
	// | FCMA                         | [19-16] |    y    |
	// |--------------------------------------------------|
	// | JSCVT                        | [15-12] |    y    |
	// |--------------------------------------------------|
	// | API                          | [11-8]  |    y    |
	// |--------------------------------------------------|
	// | APA                          | [7-4]   |    y    |
	// |--------------------------------------------------|
	// | DPB                          | [3-0]   |    y    |
	// x--------------------------------------------------x

	// if instAttrReg1&(0xf<<28) != 0 {
	// 	fmt.Println("GPI")
	// }
	if instAttrReg1&(0xf<<28) != 24 {
		f |= GPA
	}
	if instAttrReg1&(0xf<<20) != 0 {
		f |= LRCPC
	}
	if instAttrReg1&(0xf<<16) != 0 {
		f |= FCMA
	}
	if instAttrReg1&(0xf<<12) != 0 {
		f |= JSCVT
	}
	// if instAttrReg1&(0xf<<8) != 0 {
	// 	fmt.Println("API")
	// }
	// if instAttrReg1&(0xf<<4) != 0 {
	// 	fmt.Println("APA")
	// }
	if instAttrReg1&(0xf<<0) != 0 {
		f |= DCPOP
	}
	c.Arm = f
}
