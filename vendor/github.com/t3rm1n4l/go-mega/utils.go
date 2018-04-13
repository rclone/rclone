package mega

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"math/big"
	"net"
	"net/http"
	"strings"
	"time"
)

func newHttpClient(timeout time.Duration) *http.Client {
	// TODO: Need to test this out
	// Doesn't seem to work as expected
	c := &http.Client{
		Transport: &http.Transport{
			Dial: func(netw, addr string) (net.Conn, error) {
				c, err := net.DialTimeout(netw, addr, timeout)
				if err != nil {
					return nil, err
				}
				return c, nil
			},
			Proxy: http.ProxyFromEnvironment,
		},
	}
	return c
}

// bytes_to_a32 converts the byte slice b to uint32 slice considering
// the bytes to be in big endian order.
func bytes_to_a32(b []byte) []uint32 {
	length := len(b) + 3
	a := make([]uint32, length/4)
	buf := bytes.NewBuffer(b)
	for i, _ := range a {
		_ = binary.Read(buf, binary.BigEndian, &a[i])
	}

	return a
}

// a32_to_bytes converts the uint32 slice a to byte slice where each
// uint32 is decoded in big endian order.
func a32_to_bytes(a []uint32) []byte {
	buf := new(bytes.Buffer)
	buf.Grow(len(a) * 4) // To prevent reallocations in Write
	for _, v := range a {
		_ = binary.Write(buf, binary.BigEndian, v)
	}

	return buf.Bytes()
}

// base64urlencode encodes byte slice b using base64 url encoding.
// It removes `=` padding when necessary
func base64urlencode(b []byte) []byte {
	enc := base64.URLEncoding
	encSize := enc.EncodedLen(len(b))
	buf := make([]byte, encSize)
	enc.Encode(buf, b)

	paddSize := 3 - len(b)%3
	if paddSize < 3 {
		encSize -= paddSize
		buf = buf[:encSize]
	}

	return buf
}

// base64urldecode decodes the byte slice b using base64 url decoding.
// It adds required '=' padding before decoding.
func base64urldecode(b []byte) []byte {
	enc := base64.URLEncoding
	padSize := 4 - len(b)%4

	switch padSize {
	case 1:
		b = append(b, '=')
	case 2:
		b = append(b, '=', '=')
	}

	decSize := enc.DecodedLen(len(b))
	buf := make([]byte, decSize)
	n, _ := enc.Decode(buf, b)
	return buf[:n]
}

// base64_to_a32 converts base64 encoded byte slice b to uint32 slice.
func base64_to_a32(b []byte) []uint32 {
	return bytes_to_a32(base64urldecode(b))
}

// a32_to_base64 converts uint32 slice to base64 encoded byte slice.
func a32_to_base64(a []uint32) []byte {
	return base64urlencode(a32_to_bytes(a))
}

// paddnull pads byte slice b such that the size of resulting byte
// slice is a multiple of q.
func paddnull(b []byte, q int) []byte {
	if rem := len(b) % q; rem != 0 {
		l := q - rem

		for i := 0; i < l; i++ {
			b = append(b, 0)
		}
	}

	return b
}

// password_key calculates password hash from the user password.
func password_key(p string) []byte {
	a := bytes_to_a32(paddnull([]byte(p), 4))

	pkey := a32_to_bytes([]uint32{0x93C467E3, 0x7DB0C7A4, 0xD1BE3F81, 0x0152CB56})

	n := (len(a) + 3) / 4

	ciphers := make([]cipher.Block, n)

	for j := 0; j < len(a); j += 4 {
		key := []uint32{0, 0, 0, 0}
		for k := 0; k < 4; k++ {
			if j+k < len(a) {
				key[k] = a[k+j]
			}
		}
		ciphers[j/4], _ = aes.NewCipher(a32_to_bytes(key)) // Uses AES in ECB mode
	}

	for i := 65536; i > 0; i-- {
		for j := 0; j < n; j++ {
			ciphers[j].Encrypt(pkey, pkey)
		}
	}

	return pkey
}

// stringhash computes generic string hash. Uses k as the key for AES
// cipher.
func stringhash(s string, k []byte) []byte {
	a := bytes_to_a32(paddnull([]byte(s), 4))
	h := []uint32{0, 0, 0, 0}
	for i, v := range a {
		h[i&3] ^= v
	}

	hb := a32_to_bytes(h)
	cipher, _ := aes.NewCipher(k)
	for i := 16384; i > 0; i-- {
		cipher.Encrypt(hb, hb)
	}
	ha := bytes_to_a32(paddnull(hb, 4))

	return a32_to_base64([]uint32{ha[0], ha[2]})
}

// getMPI returns the length encoded Int and the next slice.
func getMPI(b []byte) (*big.Int, []byte) {
	p := new(big.Int)
	plen := (uint64(b[0])*256 + uint64(b[1]) + 7) >> 3
	p.SetBytes(b[2 : plen+2])
	b = b[plen+2:]
	return p, b
}

// getRSAKey decodes the RSA Key from the byte slice b.
func getRSAKey(b []byte) (*big.Int, *big.Int, *big.Int) {
	p, b := getMPI(b)
	q, b := getMPI(b)
	d, _ := getMPI(b)

	return p, q, d
}

// decryptRSA decrypts message m using RSA private key (p,q,d)
func decryptRSA(m, p, q, d *big.Int) []byte {
	n := new(big.Int)
	r := new(big.Int)
	n.Mul(p, q)
	r.Exp(m, d, n)

	return r.Bytes()
}

// blockDecrypt decrypts using the block cipher blk in ECB mode.
func blockDecrypt(blk cipher.Block, dst, src []byte) error {

	if len(src) > len(dst) || len(src)%blk.BlockSize() != 0 {
		return errors.New("Block decryption failed")
	}

	l := len(src) - blk.BlockSize()

	for i := 0; i <= l; i += blk.BlockSize() {
		blk.Decrypt(dst[i:], src[i:])
	}

	return nil
}

// blockEncrypt encrypts using the block cipher blk in ECB mode.
func blockEncrypt(blk cipher.Block, dst, src []byte) error {

	if len(src) > len(dst) || len(src)%blk.BlockSize() != 0 {
		return errors.New("Block encryption failed")
	}

	l := len(src) - blk.BlockSize()

	for i := 0; i <= l; i += blk.BlockSize() {
		blk.Encrypt(dst[i:], src[i:])
	}

	return nil
}

// decryptSeessionId decrypts the session id using the given private
// key.
func decryptSessionId(privk []byte, csid []byte, mk []byte) ([]byte, error) {

	block, _ := aes.NewCipher(mk)
	pk := base64urldecode(privk)
	err := blockDecrypt(block, pk, pk)
	if err != nil {
		return nil, err
	}

	c := base64urldecode(csid)

	m, _ := getMPI(c)

	p, q, d := getRSAKey(pk)
	r := decryptRSA(m, p, q, d)

	return base64urlencode(r[:43]), nil

}

// chunkSize describes a size and position of chunk
type chunkSize struct {
	position int64
	size     int
}

func getChunkSizes(size int64) (chunks []chunkSize) {
	p := int64(0)
	for i := 1; size > 0; i++ {
		var chunk int
		if i <= 8 {
			chunk = i * 131072
		} else {
			chunk = 1048576
		}
		if size < int64(chunk) {
			chunk = int(size)
		}
		chunks = append(chunks, chunkSize{position: p, size: chunk})
		p += int64(chunk)
		size -= int64(chunk)
	}
	return chunks
}

func decryptAttr(key []byte, data []byte) (attr FileAttr, err error) {
	err = EBADATTR
	block, err := aes.NewCipher(key)
	if err != nil {
		return attr, err
	}
	iv := a32_to_bytes([]uint32{0, 0, 0, 0})
	mode := cipher.NewCBCDecrypter(block, iv)
	buf := make([]byte, len(data))
	mode.CryptBlocks(buf, base64urldecode([]byte(data)))

	if string(buf[:4]) == "MEGA" {
		str := strings.TrimRight(string(buf[4:]), "\x00")
		err = json.Unmarshal([]byte(str), &attr)
	}
	return attr, err
}

func encryptAttr(key []byte, attr FileAttr) (b []byte, err error) {
	err = EBADATTR
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	data, err := json.Marshal(attr)
	if err != nil {
		return nil, err
	}
	attrib := []byte("MEGA")
	attrib = append(attrib, data...)
	attrib = paddnull(attrib, 16)

	iv := a32_to_bytes([]uint32{0, 0, 0, 0})
	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(attrib, attrib)

	b = base64urlencode(attrib)
	return b, nil
}

func randString(l int) (string, error) {
	encoding := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789AB"
	b := make([]byte, l)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	enc := base64.NewEncoding(encoding)
	d := make([]byte, enc.EncodedLen(len(b)))
	enc.Encode(d, b)
	d = d[:l]
	return string(d), nil
}
