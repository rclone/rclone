// EME (ECB-Mix-ECB or, clearer, Encrypt-Mix-Encrypt) is a wide-block
// encryption mode developed by Halevi and Rogaway.
//
// It was presented in the 2003 paper "A Parallelizable Enciphering Mode" by
// Halevi and Rogaway.
//
// EME uses multiple invocations of a block cipher to construct a new cipher
// of bigger block size (in multiples of 16 bytes, up to 2048 bytes).
package eme

import (
	"crypto/cipher"
	"log"
)

type directionConst bool

const (
	// Encrypt "inputData"
	DirectionEncrypt = directionConst(true)
	// Decrypt "inputData"
	DirectionDecrypt = directionConst(false)
)

// multByTwo - GF multiplication as specified in the EME-32 draft
func multByTwo(out []byte, in []byte) {
	if len(in) != 16 {
		panic("len must be 16")
	}
	tmp := make([]byte, 16)

	tmp[0] = 2 * in[0]
	if in[15] >= 128 {
		tmp[0] = tmp[0] ^ 135
	}
	for j := 1; j < 16; j++ {
		tmp[j] = 2 * in[j]
		if in[j-1] >= 128 {
			tmp[j] += 1
		}
	}
	copy(out, tmp)
}

func xorBlocks(out []byte, in1 []byte, in2 []byte) {
	if len(in1) != len(in2) {
		log.Panicf("len(in1)=%d is not equal to len(in2)=%d", len(in1), len(in2))
	}

	for i := range in1 {
		out[i] = in1[i] ^ in2[i]
	}
}

// aesTransform - encrypt or decrypt (according to "direction") using block
// cipher "bc" (typically AES)
func aesTransform(dst []byte, src []byte, direction directionConst, bc cipher.Block) {
	if direction == DirectionEncrypt {
		bc.Encrypt(dst, src)
		return
	} else if direction == DirectionDecrypt {
		bc.Decrypt(dst, src)
		return
	}
}

// tabulateL - calculate L_i for messages up to a length of m cipher blocks
func tabulateL(bc cipher.Block, m int) [][]byte {
	/* set L0 = 2*AESenc(K; 0) */
	eZero := make([]byte, 16)
	Li := make([]byte, 16)
	bc.Encrypt(Li, eZero)

	LTable := make([][]byte, m)
	// Allocate pool once and slice into m pieces in the loop
	pool := make([]byte, m*16)
	for i := 0; i < m; i++ {
		multByTwo(Li, Li)
		LTable[i] = pool[i*16 : (i+1)*16]
		copy(LTable[i], Li)
	}
	return LTable
}

// Transform - EME-encrypt or EME-decrypt, according to "direction"
// (defined in the constants DirectionEncrypt and DirectionDecrypt).
// The data in "inputData" is en- or decrypted with the block ciper "bc" under
// "tweak" (also known as IV).
//
// The tweak is used to randomize the encryption in the same way as an
// IV.  A use of this encryption mode envisioned by the authors of the
// algorithm was to encrypt each sector of a disk, with the tweak
// being the sector number.  If you encipher the same data with the
// same tweak you will get the same ciphertext.
//
// The result is returned in a freshly allocated slice of the same
// size as inputData.
//
// Limitations:
// * The block cipher must have block size 16 (usually AES).
// * The size of "tweak" must be 16
// * "inputData" must be a multiple of 16 bytes long
// If any of these pre-conditions are not met, the function will panic.
//
// Note that you probably don't want to call this function directly and instead
// use eme.New(), which provides conventient wrappers.
func Transform(bc cipher.Block, tweak []byte, inputData []byte, direction directionConst) []byte {
	// In the paper, the tweak is just called "T". Call it the same here to
	// make following the paper easy.
	T := tweak
	// In the paper, the plaintext data is called "P" and the ciphertext is
	// called "C". Because encryption and decryption are virtually identical,
	// we share the code and always call the input data "P" and the output data
	// "C", regardless of the direction.
	P := inputData

	if bc.BlockSize() != 16 {
		log.Panicf("Using a block size other than 16 is not implemented")
	}
	if len(T) != 16 {
		log.Panicf("Tweak must be 16 bytes long, is %d", len(T))
	}
	if len(P)%16 != 0 {
		log.Panicf("Data P must be a multiple of 16 long, is %d", len(P))
	}
	m := len(P) / 16
	if m == 0 || m > 16*8 {
		log.Panicf("EME operates on 1 to %d block-cipher blocks, you passed %d", 16*8, m)
	}

	C := make([]byte, len(P))

	LTable := tabulateL(bc, m)

	PPj := make([]byte, 16)
	for j := 0; j < m; j++ {
		Pj := P[j*16 : (j+1)*16]
		/* PPj = 2**(j-1)*L xor Pj */
		xorBlocks(PPj, Pj, LTable[j])
		/* PPPj = AESenc(K; PPj) */
		aesTransform(C[j*16:(j+1)*16], PPj, direction, bc)
	}

	/* MP =(xorSum PPPj) xor T */
	MP := make([]byte, 16)
	xorBlocks(MP, C[0:16], T)
	for j := 1; j < m; j++ {
		xorBlocks(MP, MP, C[j*16:(j+1)*16])
	}

	/* MC = AESenc(K; MP) */
	MC := make([]byte, 16)
	aesTransform(MC, MP, direction, bc)

	/* M = MP xor MC */
	M := make([]byte, 16)
	xorBlocks(M, MP, MC)
	CCCj := make([]byte, 16)
	for j := 1; j < m; j++ {
		multByTwo(M, M)
		/* CCCj = 2**(j-1)*M xor PPPj */
		xorBlocks(CCCj, C[j*16:(j+1)*16], M)
		copy(C[j*16:(j+1)*16], CCCj)
	}

	/* CCC1 = (xorSum CCCj) xor T xor MC */
	CCC1 := make([]byte, 16)
	xorBlocks(CCC1, MC, T)
	for j := 1; j < m; j++ {
		xorBlocks(CCC1, CCC1, C[j*16:(j+1)*16])
	}
	copy(C[0:16], CCC1)

	for j := 0; j < m; j++ {
		/* CCj = AES-enc(K; CCCj) */
		aesTransform(C[j*16:(j+1)*16], C[j*16:(j+1)*16], direction, bc)
		/* Cj = 2**(j-1)*L xor CCj */
		xorBlocks(C[j*16:(j+1)*16], C[j*16:(j+1)*16], LTable[j])
	}

	return C
}

// EMECipher provides EME-Encryption and -Decryption functions that are more
// convenient than calling Transform directly.
type EMECipher struct {
	bc cipher.Block
}

// New returns a new EMECipher object. "bc" must have a block size of 16,
// or subsequent calls to Encrypt and Decrypt will panic.
func New(bc cipher.Block) *EMECipher {
	return &EMECipher{
		bc: bc,
	}
}

// Encrypt is equivalent to calling Transform with direction=DirectionEncrypt.
func (e *EMECipher) Encrypt(tweak []byte, inputData []byte) []byte {
	return Transform(e.bc, tweak, inputData, DirectionEncrypt)
}

// Decrypt is equivalent to calling Transform with direction=DirectionDecrypt.
func (e *EMECipher) Decrypt(tweak []byte, inputData []byte) []byte {
	return Transform(e.bc, tweak, inputData, DirectionDecrypt)
}
