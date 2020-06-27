// Copyright (c) 2018 Igneous Systems
//   MIT License
//
//   Permission is hereby granted, free of charge, to any person obtaining a copy
//   of this software and associated documentation files (the "Software"), to deal
//   in the Software without restriction, including without limitation the rights
//   to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
//   copies of the Software, and to permit persons to whom the Software is
//   furnished to do so, subject to the following conditions:
//
//   The above copyright notice and this permission notice shall be included in all
//   copies or substantial portions of the Software.
//
//   THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
//   IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
//   FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
//   AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
//   LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
//   OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
//   SOFTWARE.

// Copyright (c) 2020 MinIO Inc. All rights reserved.
//   Use of this source code is governed by a license that can be
//   found in the LICENSE file.

// This is the AVX2 implementation of the MD5 block function (8-way parallel)

// block8(state *uint64, base uintptr, bufs *int32, cache *byte, n int)
TEXT ·block8(SB), 4, $0-40
	MOVQ state+0(FP), BX
	MOVQ base+8(FP), SI
	MOVQ bufs+16(FP), AX
	MOVQ cache+24(FP), CX
	MOVQ n+32(FP), DX
	MOVQ ·avx256md5consts+0(SB), DI

	// Align cache (which is stack allocated by the compiler)
	// to a 256 bit boundary (ymm register alignment)
	// The cache8 type is deliberately oversized to permit this.
	ADDQ $31, CX
	ANDB $-32, CL

#define a Y0
#define b Y1
#define c Y2
#define d Y3

#define sa Y4
#define sb Y5
#define sc Y6
#define sd Y7

#define tmp  Y8
#define tmp2 Y9

#define mask Y10
#define off  Y11

#define ones Y12

#define rtmp1  Y13
#define rtmp2  Y14

#define mem   Y15

#define dig    BX
#define cache  CX
#define count  DX
#define base   SI
#define consts DI

#define prepmask \
	VXORPS   mask, mask, mask \
	VPCMPGTD mask, off, mask

#define prep(index) \
	VMOVAPD    mask, rtmp2                      \
	VPGATHERDD rtmp2, index*4(base)(off*1), mem

#define load(index) \
	VMOVAPD index*32(cache), mem

#define store(index) \
	VMOVAPD mem, index*32(cache)

#define roll(shift, a) \
	VPSLLD $shift, a, rtmp1 \
	VPSRLD $32-shift, a, a  \
	VORPS  rtmp1, a, a

#define ROUND1(a, b, c, d, index, const, shift) \
	VXORPS  c, tmp, tmp            \
	VPADDD  32*const(consts), a, a \
	VPADDD  mem, a, a              \
	VANDPS  b, tmp, tmp            \
	VXORPS  d, tmp, tmp            \
	prep(index)                    \
	VPADDD  tmp, a, a              \
	roll(shift,a)                  \
	VMOVAPD c, tmp                 \
	VPADDD  b, a, a

#define ROUND1load(a, b, c, d, index, const, shift) \
	VXORPS  c, tmp, tmp            \
	VPADDD  32*const(consts), a, a \
	VPADDD  mem, a, a              \
	VANDPS  b, tmp, tmp            \
	VXORPS  d, tmp, tmp            \
	load(index)                    \
	VPADDD  tmp, a, a              \
	roll(shift,a)                  \
	VMOVAPD c, tmp                 \
	VPADDD  b, a, a

#define ROUND2(a, b, c, d, index, const, shift) \
	VPADDD  32*const(consts), a, a \
	VPADDD  mem, a, a              \
	VANDPS  b, tmp2, tmp2          \
	VANDNPS c, tmp, tmp            \
	load(index)                    \
	VORPS   tmp, tmp2, tmp2        \
	VMOVAPD c, tmp                 \
	VPADDD  tmp2, a, a             \
	VMOVAPD c, tmp2                \
	roll(shift,a)                  \
	VPADDD  b, a, a

#define ROUND3(a, b, c, d, index, const, shift) \
	VPADDD  32*const(consts), a, a \
	VPADDD  mem, a, a              \
	load(index)                    \
	VXORPS  d, tmp, tmp            \
	VXORPS  b, tmp, tmp            \
	VPADDD  tmp, a, a              \
	roll(shift,a)                  \
	VMOVAPD b, tmp                 \
	VPADDD  b, a, a

#define ROUND4(a, b, c, d, index, const, shift) \
	VPADDD 32*const(consts), a, a \
	VPADDD mem, a, a              \
	VORPS  b, tmp, tmp            \
	VXORPS c, tmp, tmp            \
	VPADDD tmp, a, a              \
	load(index)                   \
	roll(shift,a)                 \
	VXORPS c, ones, tmp           \
	VPADDD b, a, a

	// load digest into state registers
	VMOVUPD (dig), a
	VMOVUPD 32(dig), b
	VMOVUPD 64(dig), c
	VMOVUPD 96(dig), d

	// load source buffer offsets
	VMOVUPD (AX), off

	prepmask
	VPCMPEQD ones, ones, ones

loop:
	VMOVAPD a, sa
	VMOVAPD b, sb
	VMOVAPD c, sc
	VMOVAPD d, sd

	prep(0)
	VMOVAPD d, tmp
	store(0)

	ROUND1(a,b,c,d, 1,0x00, 7)
	store(1)
	ROUND1(d,a,b,c, 2,0x01,12)
	store(2)
	ROUND1(c,d,a,b, 3,0x02,17)
	store(3)
	ROUND1(b,c,d,a, 4,0x03,22)
	store(4)
	ROUND1(a,b,c,d, 5,0x04, 7)
	store(5)
	ROUND1(d,a,b,c, 6,0x05,12)
	store(6)
	ROUND1(c,d,a,b, 7,0x06,17)
	store(7)
	ROUND1(b,c,d,a, 8,0x07,22)
	store(8)
	ROUND1(a,b,c,d, 9,0x08, 7)
	store(9)
	ROUND1(d,a,b,c,10,0x09,12)
	store(10)
	ROUND1(c,d,a,b,11,0x0a,17)
	store(11)
	ROUND1(b,c,d,a,12,0x0b,22)
	store(12)
	ROUND1(a,b,c,d,13,0x0c, 7)
	store(13)
	ROUND1(d,a,b,c,14,0x0d,12)
	store(14)
	ROUND1(c,d,a,b,15,0x0e,17)
	store(15)
	ROUND1load(b,c,d,a, 1,0x0f,22)

	VMOVAPD d, tmp
	VMOVAPD d, tmp2

	ROUND2(a,b,c,d, 6,0x10, 5)
	ROUND2(d,a,b,c,11,0x11, 9)
	ROUND2(c,d,a,b, 0,0x12,14)
	ROUND2(b,c,d,a, 5,0x13,20)
	ROUND2(a,b,c,d,10,0x14, 5)
	ROUND2(d,a,b,c,15,0x15, 9)
	ROUND2(c,d,a,b, 4,0x16,14)
	ROUND2(b,c,d,a, 9,0x17,20)
	ROUND2(a,b,c,d,14,0x18, 5)
	ROUND2(d,a,b,c, 3,0x19, 9)
	ROUND2(c,d,a,b, 8,0x1a,14)
	ROUND2(b,c,d,a,13,0x1b,20)
	ROUND2(a,b,c,d, 2,0x1c, 5)
	ROUND2(d,a,b,c, 7,0x1d, 9)
	ROUND2(c,d,a,b,12,0x1e,14)
	ROUND2(b,c,d,a, 0,0x1f,20)

	load(5)
	VMOVAPD c, tmp

	ROUND3(a,b,c,d, 8,0x20, 4)
	ROUND3(d,a,b,c,11,0x21,11)
	ROUND3(c,d,a,b,14,0x22,16)
	ROUND3(b,c,d,a, 1,0x23,23)
	ROUND3(a,b,c,d, 4,0x24, 4)
	ROUND3(d,a,b,c, 7,0x25,11)
	ROUND3(c,d,a,b,10,0x26,16)
	ROUND3(b,c,d,a,13,0x27,23)
	ROUND3(a,b,c,d, 0,0x28, 4)
	ROUND3(d,a,b,c, 3,0x29,11)
	ROUND3(c,d,a,b, 6,0x2a,16)
	ROUND3(b,c,d,a, 9,0x2b,23)
	ROUND3(a,b,c,d,12,0x2c, 4)
	ROUND3(d,a,b,c,15,0x2d,11)
	ROUND3(c,d,a,b, 2,0x2e,16)
	ROUND3(b,c,d,a, 0,0x2f,23)

	load(0)
	VXORPS d, ones, tmp

	ROUND4(a,b,c,d, 7,0x30, 6)
	ROUND4(d,a,b,c,14,0x31,10)
	ROUND4(c,d,a,b, 5,0x32,15)
	ROUND4(b,c,d,a,12,0x33,21)
	ROUND4(a,b,c,d, 3,0x34, 6)
	ROUND4(d,a,b,c,10,0x35,10)
	ROUND4(c,d,a,b, 1,0x36,15)
	ROUND4(b,c,d,a, 8,0x37,21)
	ROUND4(a,b,c,d,15,0x38, 6)
	ROUND4(d,a,b,c, 6,0x39,10)
	ROUND4(c,d,a,b,13,0x3a,15)
	ROUND4(b,c,d,a, 4,0x3b,21)
	ROUND4(a,b,c,d,11,0x3c, 6)
	ROUND4(d,a,b,c, 2,0x3d,10)
	ROUND4(c,d,a,b, 9,0x3e,15)
	ROUND4(b,c,d,a, 0,0x3f,21)

	VPADDD sa, a, a
	VPADDD sb, b, b
	VPADDD sc, c, c
	VPADDD sd, d, d

	LEAQ 64(base), base
	SUBQ $64, count
	JNE  loop

	VMOVUPD a, (dig)
	VMOVUPD b, 32(dig)
	VMOVUPD c, 64(dig)
	VMOVUPD d, 96(dig)

	VZEROUPPER
	RET
