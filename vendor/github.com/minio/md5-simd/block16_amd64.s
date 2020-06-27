// Copyright (c) 2020 MinIO Inc. All rights reserved.
// Use of this source code is governed by a license that can be
// found in the LICENSE file.

// This is the AVX512 implementation of the MD5 block function (16-way parallel)

#define prep(index) \
	KMOVQ	   kmask, ktmp					    \
	VPGATHERDD index*4(base)(ptrs*1), ktmp, mem

#define ROUND1(a, b, c, d, index, const, shift) \
	VXORPS  c, tmp, tmp            \
	VPADDD  64*const(consts), a, a \
	VPADDD  mem, a, a              \
	VPTERNLOGD $0x6C, b, d, tmp    \
	prep(index)                    \
	VPADDD  tmp, a, a              \
	VPROLD $shift, a, a            \
	VMOVAPD c, tmp                 \
	VPADDD  b, a, a

#define ROUND1noload(a, b, c, d, const, shift) \
	VXORPS  c, tmp, tmp            \
	VPADDD  64*const(consts), a, a \
	VPADDD  mem, a, a              \
	VPTERNLOGD $0x6C, b, d, tmp    \
	VPADDD  tmp, a, a              \
	VPROLD $shift, a, a            \
	VMOVAPD c, tmp                 \
	VPADDD  b, a, a

#define ROUND2(a, b, c, d, zreg, const, shift) \
	VPADDD  64*const(consts), a, a \
	VPADDD  zreg, a, a             \
	VANDNPS c, tmp, tmp            \
	VPTERNLOGD $0xEC, b, tmp, tmp2 \
	VMOVAPD c, tmp                 \
	VPADDD  tmp2, a, a             \
	VMOVAPD c, tmp2                \
	VPROLD $shift, a, a            \
	VPADDD  b, a, a

#define ROUND3(a, b, c, d, zreg, const, shift) \
	VPADDD  64*const(consts), a, a \
	VPADDD  zreg, a, a             \
	VPTERNLOGD $0x96, b, d, tmp    \
	VPADDD  tmp, a, a              \
	VPROLD $shift, a, a            \
	VMOVAPD b, tmp                 \
	VPADDD  b, a, a

#define ROUND4(a, b, c, d, zreg, const, shift) \
	VPADDD 64*const(consts), a, a \
	VPADDD zreg, a, a             \
	VPTERNLOGD $0x36, b, c, tmp   \
	VPADDD tmp, a, a              \
	VPROLD $shift, a, a           \
	VXORPS c, ones, tmp           \
	VPADDD b, a, a

TEXT ·block16(SB),4,$0-40

    MOVQ  state+0(FP), BX
    MOVQ  base+8(FP), SI
    MOVQ  ptrs+16(FP), AX
    KMOVQ mask+24(FP), K1
    MOVQ  n+32(FP), DX
    MOVQ  ·avx512md5consts+0(SB), DI

#define a Z0
#define b Z1
#define c Z2
#define d Z3

#define sa Z4
#define sb Z5
#define sc Z6
#define sd Z7

#define tmp       Z8
#define tmp2      Z9
#define ptrs     Z10
#define ones     Z12
#define mem      Z15

#define kmask  K1
#define ktmp   K3

// ----------------------------------------------------------
// Registers Z16 through to Z31 are used for caching purposes
// ----------------------------------------------------------


#define dig    BX
#define count  DX
#define base   SI
#define consts DI

	// load digest into state registers
	VMOVUPD (dig), a
	VMOVUPD 0x40(dig), b
	VMOVUPD 0x80(dig), c
	VMOVUPD 0xc0(dig), d

	// load source pointers
	VMOVUPD 0x00(AX), ptrs

	MOVQ $-1, AX
	VPBROADCASTQ AX, ones

loop:
	VMOVAPD a, sa
	VMOVAPD b, sb
	VMOVAPD c, sc
	VMOVAPD d, sd

	prep(0)
	VMOVAPD d, tmp
	VMOVAPD mem, Z16

	ROUND1(a,b,c,d, 1,0x00, 7)
	VMOVAPD mem, Z17
	ROUND1(d,a,b,c, 2,0x01,12)
	VMOVAPD mem, Z18
	ROUND1(c,d,a,b, 3,0x02,17)
	VMOVAPD mem, Z19
	ROUND1(b,c,d,a, 4,0x03,22)
	VMOVAPD mem, Z20
	ROUND1(a,b,c,d, 5,0x04, 7)
	VMOVAPD mem, Z21
	ROUND1(d,a,b,c, 6,0x05,12)
	VMOVAPD mem, Z22
	ROUND1(c,d,a,b, 7,0x06,17)
	VMOVAPD mem, Z23
	ROUND1(b,c,d,a, 8,0x07,22)
	VMOVAPD mem, Z24
	ROUND1(a,b,c,d, 9,0x08, 7)
	VMOVAPD mem, Z25
	ROUND1(d,a,b,c,10,0x09,12)
	VMOVAPD mem, Z26
	ROUND1(c,d,a,b,11,0x0a,17)
	VMOVAPD mem, Z27
	ROUND1(b,c,d,a,12,0x0b,22)
	VMOVAPD mem, Z28
	ROUND1(a,b,c,d,13,0x0c, 7)
	VMOVAPD mem, Z29
	ROUND1(d,a,b,c,14,0x0d,12)
	VMOVAPD mem, Z30
	ROUND1(c,d,a,b,15,0x0e,17)
	VMOVAPD mem, Z31

	ROUND1noload(b,c,d,a, 0x0f,22)

	VMOVAPD d, tmp
	VMOVAPD d, tmp2

	ROUND2(a,b,c,d, Z17,0x10, 5)
	ROUND2(d,a,b,c, Z22,0x11, 9)
	ROUND2(c,d,a,b, Z27,0x12,14)
	ROUND2(b,c,d,a, Z16,0x13,20)
	ROUND2(a,b,c,d, Z21,0x14, 5)
	ROUND2(d,a,b,c, Z26,0x15, 9)
	ROUND2(c,d,a,b, Z31,0x16,14)
	ROUND2(b,c,d,a, Z20,0x17,20)
	ROUND2(a,b,c,d, Z25,0x18, 5)
	ROUND2(d,a,b,c, Z30,0x19, 9)
	ROUND2(c,d,a,b, Z19,0x1a,14)
	ROUND2(b,c,d,a, Z24,0x1b,20)
	ROUND2(a,b,c,d, Z29,0x1c, 5)
	ROUND2(d,a,b,c, Z18,0x1d, 9)
	ROUND2(c,d,a,b, Z23,0x1e,14)
	ROUND2(b,c,d,a, Z28,0x1f,20)

	VMOVAPD c, tmp

	ROUND3(a,b,c,d, Z21,0x20, 4)
	ROUND3(d,a,b,c, Z24,0x21,11)
	ROUND3(c,d,a,b, Z27,0x22,16)
	ROUND3(b,c,d,a, Z30,0x23,23)
	ROUND3(a,b,c,d, Z17,0x24, 4)
	ROUND3(d,a,b,c, Z20,0x25,11)
	ROUND3(c,d,a,b, Z23,0x26,16)
	ROUND3(b,c,d,a, Z26,0x27,23)
	ROUND3(a,b,c,d, Z29,0x28, 4)
	ROUND3(d,a,b,c, Z16,0x29,11)
	ROUND3(c,d,a,b, Z19,0x2a,16)
	ROUND3(b,c,d,a, Z22,0x2b,23)
	ROUND3(a,b,c,d, Z25,0x2c, 4)
	ROUND3(d,a,b,c, Z28,0x2d,11)
	ROUND3(c,d,a,b, Z31,0x2e,16)
	ROUND3(b,c,d,a, Z18,0x2f,23)

	VXORPS d, ones, tmp

	ROUND4(a,b,c,d, Z16,0x30, 6)
	ROUND4(d,a,b,c, Z23,0x31,10)
	ROUND4(c,d,a,b, Z30,0x32,15)
	ROUND4(b,c,d,a, Z21,0x33,21)
	ROUND4(a,b,c,d, Z28,0x34, 6)
	ROUND4(d,a,b,c, Z19,0x35,10)
	ROUND4(c,d,a,b, Z26,0x36,15)
	ROUND4(b,c,d,a, Z17,0x37,21)
	ROUND4(a,b,c,d, Z24,0x38, 6)
	ROUND4(d,a,b,c, Z31,0x39,10)
	ROUND4(c,d,a,b, Z22,0x3a,15)
	ROUND4(b,c,d,a, Z29,0x3b,21)
	ROUND4(a,b,c,d, Z20,0x3c, 6)
	ROUND4(d,a,b,c, Z27,0x3d,10)
	ROUND4(c,d,a,b, Z18,0x3e,15)
	ROUND4(b,c,d,a, Z25,0x3f,21)

	VPADDD sa, a, a
	VPADDD sb, b, b
	VPADDD sc, c, c
	VPADDD sd, d, d

	LEAQ 64(base), base
	SUBQ $64, count
	JNE  loop

	VMOVUPD a, (dig)
	VMOVUPD b, 0x40(dig)
	VMOVUPD c, 0x80(dig)
	VMOVUPD d, 0xc0(dig)

	VZEROUPPER
	RET
