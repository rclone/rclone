package ccm

import (
	"crypto/cipher"
)

// CBC-MAC implementation
type mac struct {
	ci []byte
	p  int
	c  cipher.Block
}

func newMAC(c cipher.Block) *mac {
	return &mac{
		c:  c,
		ci: make([]byte, c.BlockSize()),
	}
}

func (m *mac) Reset() {
	for i := range m.ci {
		m.ci[i] = 0
	}
	m.p = 0
}

func (m *mac) Write(p []byte) (n int, err error) {
	for _, c := range p {
		if m.p >= len(m.ci) {
			m.c.Encrypt(m.ci, m.ci)
			m.p = 0
		}
		m.ci[m.p] ^= c
		m.p++
	}
	return len(p), nil
}

// PadZero emulates zero byte padding.
func (m *mac) PadZero() {
	if m.p != 0 {
		m.c.Encrypt(m.ci, m.ci)
		m.p = 0
	}
}

func (m *mac) Sum(in []byte) []byte {
	return append(in, m.ci...)
}

func (m *mac) Size() int { return len(m.ci) }

func (m *mac) BlockSize() int { return 16 }
