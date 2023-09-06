package consts

import (
	"os"

	"github.com/klauspost/cpuid/v2"
)

var (
	HasAVX2 = cpuid.CPU.Has(cpuid.AVX2) &&
		os.Getenv("BLAKE3_DISABLE_AVX2") == "" &&
		os.Getenv("BLAKE3_PUREGO") == ""

	HasSSE41 = cpuid.CPU.Has(cpuid.SSE4) &&
		os.Getenv("BLAKE3_DISABLE_SSE41") == "" &&
		os.Getenv("BLAKE3_PUREGO") == ""
)
