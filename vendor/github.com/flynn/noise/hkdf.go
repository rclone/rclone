package noise

import (
	"crypto/hmac"
	"hash"
)

func hkdf(h func() hash.Hash, outputs int, out1, out2, out3, chainingKey, inputKeyMaterial []byte) ([]byte, []byte, []byte) {
	if len(out1) > 0 {
		panic("len(out1) > 0")
	}
	if len(out2) > 0 {
		panic("len(out2) > 0")
	}
	if len(out3) > 0 {
		panic("len(out3) > 0")
	}
	if outputs > 3 {
		panic("outputs > 3")
	}

	tempMAC := hmac.New(h, chainingKey)
	tempMAC.Write(inputKeyMaterial)
	tempKey := tempMAC.Sum(out2)

	out1MAC := hmac.New(h, tempKey)
	out1MAC.Write([]byte{0x01})
	out1 = out1MAC.Sum(out1)

	if outputs == 1 {
		return out1, nil, nil
	}

	out2MAC := hmac.New(h, tempKey)
	out2MAC.Write(out1)
	out2MAC.Write([]byte{0x02})
	out2 = out2MAC.Sum(out2)

	if outputs == 2 {
		return out1, out2, nil
	}

	out3MAC := hmac.New(h, tempKey)
	out3MAC.Write(out2)
	out3MAC.Write([]byte{0x03})
	out3 = out3MAC.Sum(out3)

	return out1, out2, out3
}
