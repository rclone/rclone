// The MIT License (MIT)
//
// Copyright (C) 2016-2017 Vivint, Inc.
// Copyright (c) 2015 Klaus Post
// Copyright (c) 2015 Backblaze
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.

/*
The corresponding C implementations:

void addmul(
	uint8_t * restrict lowhigh,
	uint8_t * restrict in,
	uint8_t * restrict out,
	int n
) {
	for(int i = 0; i < n; i++){
		int value = in[i];
		int low   = value & 15;
		int high  = value >> 4;
		out[i] = out[i] ^ lowhigh[low] ^ lowhigh[high+16];
	}
}

void addmulSSSE3(
	uint8_t * restrict lowhigh,
	uint8_t * restrict in,
	uint8_t * restrict out,
	int n
) {
	int i = 0;

	__m128i lotbl = _mm_loadu_si128((__m128i*)(&lowhigh[0]));
	__m128i hitbl = _mm_loadu_si128((__m128i*)(&lowhigh[16]));

	__m128i lomask = _mm_set1_epi8(0xF);

	#pragma nounroll
	for(i = 0; i < (n/16)*16; i += 16){
		__m128i input8  = _mm_loadu_si128((__m128i*)(&in[i]));
		__m128i output8 = _mm_loadu_si128((__m128i*)(&out[i]));

		__m128i lo8 = _mm_and_si128(lomask, input8);
		__m128i hi8 = _mm_and_si128(lomask, _mm_srli_si128(input8, 4)); // simulate shrli epi8

		output8 = _mm_xor_si128(output8, _mm_shuffle_epi8(lotbl, lo8));
		output8 = _mm_xor_si128(output8, _mm_shuffle_epi8(hitbl, hi8));

		_mm_storeu_si128((__m128i*)(&out[i]), output8);
	}
}
*/

#include "textflag.h"
DATA  nybble_mask<>+0x00(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x08(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x10(SB)/8, $0x0F0F0F0F0F0F0F0F
DATA  nybble_mask<>+0x18(SB)/8, $0x0F0F0F0F0F0F0F0F
GLOBL nybble_mask<>(SB), (NOPTR+RODATA), $32

#define LOWHIGH  DI
#define LOW   X8
#define HIGH  X9
#define IN    SI
#define OUT   DX
#define INDEX AX

#define LEN   CX
#define LEN16 R8 // LEN16 = (LEN / 16) * 16

#define LOMASK X7 // LOMASK = repeated 15 
// X0-X5 temps

// func addmulSSSE3(lowhigh *[2][16]byte, in, out *byte, len int)
TEXT ·addmulSSSE3(SB), 7, $0
	MOVQ _in+8(FP),   IN
	MOVQ _out+16(FP), OUT
	MOVQ _len+24(FP), LEN

	MOVQ LEN,  LEN16
	ANDQ $-16, LEN16

	JLE start_slow // if LEN16 == 0 { goto done }
	
	MOVQ _lohi+0(FP), LOWHIGH
	MOVOU    (LOWHIGH), LOW
	MOVOU  16(LOWHIGH), HIGH
	
	MOVOU  nybble_mask<>(SB), LOMASK
	XORQ   INDEX, INDEX // INDEX = 0

loop16:
	MOVOU  (IN)(INDEX*1),  X0 // X0 = INPUT[INDEX]
	MOVOU  LOW,  X4            // X4 = copy(LOW)
	MOVOU  (OUT)(INDEX*1), X2 // X2 = OUT[INDEX]
	MOVOU  X0, X1              // X0 = input[index] & 15
	MOVOU  HIGH, X5            // X5 = copy(HIGH)
	
	PAND   LOMASK, X0
	PSRLQ  $4, X1              // X1 = input[index]
	PSHUFB X0, X4             // X4 = LOW[X0]

	PAND   LOMASK, X1         // X1 = input[index] >> 4
	PSHUFB X1, X5            // X5 = HIGH[X1]
	PXOR   X4, X2            // X2 = OUT[INDEX] ^ X4 ^ X5
	PXOR   X5, X2

	MOVOU X2, 0(OUT)(INDEX*1)
	
	ADDQ $16,   INDEX
	CMPQ LEN16, INDEX // INDEX < LEN16
	JG loop16

start_slow:
	MOVQ  _len+32(FP), LOWHIGH
	MOVQ LEN16, INDEX
	CMPQ LEN, INDEX
	JLE done

loop1:
	MOVBQZX (IN)(INDEX*1),   R9  // R9  := in[index]
	MOVBQZX (LOWHIGH)(R9*1), R10 // R10 := multiply[R9]
	XORB R10B, (OUT)(INDEX*1)    // out[index] ^= R10
	INCQ INDEX
	CMPQ LEN, INDEX
	JG loop1

done:
	RET

#undef LOWHIGH
#undef LOW
#undef HIGH
#undef IN
#undef OUT
#undef LEN
#undef INDEX
#undef LEN16
#undef LOMASK

// func addmulAVX2(lowhigh *[2][16]byte, in, out *byte, len int)
TEXT ·addmulAVX2(SB), 7, $0
	MOVQ  low+0(FP), SI     // SI: &lowhigh
	MOVOU (SI),   X6        // X6: low
	MOVOU 16(SI), X7        // X7: high
	
	MOVQ  $15, BX           // BX: low mask
	MOVQ  BX, X5
		
	MOVQ  len+24(FP), R9 // R9: len(in), len(out)

	LONG $0x384de3c4; WORD $0x01f6 // VINSERTI128 YMM6, YMM6, XMM6, 1 ; low
	LONG $0x3845e3c4; WORD $0x01ff // VINSERTI128 YMM7, YMM7, XMM7, 1 ; high
	LONG $0x787d62c4; BYTE $0xc5   // VPBROADCASTB YMM8, XMM5         ; X8: lomask (unpacked)

	SHRQ  $5, R9         // len(in) / 32
	MOVQ  out+16(FP), DX // DX: &out
	MOVQ  in+8(FP), SI   // R11: &in
	TESTQ R9, R9
	JZ    done_xor_avx2

loopback_xor_avx2:
	LONG $0x066ffec5             // VMOVDQU YMM0, [rsi]
	LONG $0x226ffec5             // VMOVDQU YMM4, [rdx]
	LONG $0xd073f5c5; BYTE $0x04 // VPSRLQ  YMM1, YMM0, 4   ; X1: high input
	LONG $0xdb7dc1c4; BYTE $0xc0 // VPAND   YMM0, YMM0, YMM8      ; X0: low input
	LONG $0xdb75c1c4; BYTE $0xc8 // VPAND   YMM1, YMM1, YMM8      ; X1: high input
	LONG $0x004de2c4; BYTE $0xd0 // VPSHUFB  YMM2, YMM6, YMM0   ; X2: mul low part
	LONG $0x0045e2c4; BYTE $0xd9 // VPSHUFB  YMM3, YMM7, YMM1   ; X2: mul high part
	LONG $0xdbefedc5             // VPXOR   YMM3, YMM2, YMM3    ; X3: Result
	LONG $0xe4efe5c5             // VPXOR   YMM4, YMM3, YMM4    ; X4: Result
	LONG $0x227ffec5             // VMOVDQU [rdx], YMM4

	ADDQ $32, SI           // in+=32
	ADDQ $32, DX           // out+=32
	SUBQ $1, R9
	JNZ  loopback_xor_avx2

done_xor_avx2:
	// VZEROUPPER
	BYTE $0xc5; BYTE $0xf8; BYTE $0x77
	RET
