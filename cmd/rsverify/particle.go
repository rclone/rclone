package main

import (
	"bytes"

	"github.com/rclone/rclone/backend/rs"
)

// tryParticleFooter parses the last FooterSize bytes as an EC footer and checks payload CRC.
func tryParticleFooter(raw []byte) (payload []byte, ft *rs.Footer, ok bool) {
	if len(raw) < rs.FooterSize {
		return nil, nil, false
	}
	tail := raw[len(raw)-rs.FooterSize:]
	ft, err := rs.ParseFooter(tail)
	if err != nil {
		return nil, nil, false
	}
	payload = raw[:len(raw)-rs.FooterSize]
	if rs.CRC32C(payload) != ft.PayloadCRC32C {
		return nil, nil, false
	}
	return payload, ft, true
}

func algorithmIsRS(ft *rs.Footer) bool {
	return bytes.Equal(ft.Algorithm[:], rs.AlgorithmRS[:])
}
