package utils

import (
	"encoding/binary"
	"unsafe"
)

func SliceToArray32(bytes []byte) *[32]uint8 { return (*[32]uint8)(unsafe.Pointer(&bytes[0])) }
func SliceToArray64(bytes []byte) *[64]uint8 { return (*[64]uint8)(unsafe.Pointer(&bytes[0])) }

func BytesToWords(bytes *[64]uint8, words *[16]uint32) {
	words[0] = binary.LittleEndian.Uint32(bytes[0*4:])
	words[1] = binary.LittleEndian.Uint32(bytes[1*4:])
	words[2] = binary.LittleEndian.Uint32(bytes[2*4:])
	words[3] = binary.LittleEndian.Uint32(bytes[3*4:])
	words[4] = binary.LittleEndian.Uint32(bytes[4*4:])
	words[5] = binary.LittleEndian.Uint32(bytes[5*4:])
	words[6] = binary.LittleEndian.Uint32(bytes[6*4:])
	words[7] = binary.LittleEndian.Uint32(bytes[7*4:])
	words[8] = binary.LittleEndian.Uint32(bytes[8*4:])
	words[9] = binary.LittleEndian.Uint32(bytes[9*4:])
	words[10] = binary.LittleEndian.Uint32(bytes[10*4:])
	words[11] = binary.LittleEndian.Uint32(bytes[11*4:])
	words[12] = binary.LittleEndian.Uint32(bytes[12*4:])
	words[13] = binary.LittleEndian.Uint32(bytes[13*4:])
	words[14] = binary.LittleEndian.Uint32(bytes[14*4:])
	words[15] = binary.LittleEndian.Uint32(bytes[15*4:])
}

func WordsToBytes(words *[16]uint32, bytes []byte) {
	bytes = bytes[:64]
	binary.LittleEndian.PutUint32(bytes[0*4:1*4], words[0])
	binary.LittleEndian.PutUint32(bytes[1*4:2*4], words[1])
	binary.LittleEndian.PutUint32(bytes[2*4:3*4], words[2])
	binary.LittleEndian.PutUint32(bytes[3*4:4*4], words[3])
	binary.LittleEndian.PutUint32(bytes[4*4:5*4], words[4])
	binary.LittleEndian.PutUint32(bytes[5*4:6*4], words[5])
	binary.LittleEndian.PutUint32(bytes[6*4:7*4], words[6])
	binary.LittleEndian.PutUint32(bytes[7*4:8*4], words[7])
	binary.LittleEndian.PutUint32(bytes[8*4:9*4], words[8])
	binary.LittleEndian.PutUint32(bytes[9*4:10*4], words[9])
	binary.LittleEndian.PutUint32(bytes[10*4:11*4], words[10])
	binary.LittleEndian.PutUint32(bytes[11*4:12*4], words[11])
	binary.LittleEndian.PutUint32(bytes[12*4:13*4], words[12])
	binary.LittleEndian.PutUint32(bytes[13*4:14*4], words[13])
	binary.LittleEndian.PutUint32(bytes[14*4:15*4], words[14])
	binary.LittleEndian.PutUint32(bytes[15*4:16*4], words[15])
}

func KeyFromBytes(key []byte, out *[8]uint32) {
	key = key[:32]
	out[0] = binary.LittleEndian.Uint32(key[0:])
	out[1] = binary.LittleEndian.Uint32(key[4:])
	out[2] = binary.LittleEndian.Uint32(key[8:])
	out[3] = binary.LittleEndian.Uint32(key[12:])
	out[4] = binary.LittleEndian.Uint32(key[16:])
	out[5] = binary.LittleEndian.Uint32(key[20:])
	out[6] = binary.LittleEndian.Uint32(key[24:])
	out[7] = binary.LittleEndian.Uint32(key[28:])
}
