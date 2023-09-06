// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"encoding/binary"
	"errors"
)

type fieldType int

const (
	fieldEOS            fieldType = 0
	fieldLocation       fieldType = 1
	fieldIdentifier     fieldType = 2
	fieldVerificationID fieldType = 4
	fieldSignature      fieldType = 6
)

const (
	version byte = 2
)

type packet struct {
	fieldType fieldType
	data      []byte
}

// Serialize converts macaroon to binary format.
func (m *Macaroon) Serialize() (data []byte) {
	// Start data from version int
	data = append(data, version)

	// Serialize Identity
	data = serializePacket(data, packet{
		fieldType: fieldIdentifier,
		data:      m.head,
	})
	data = append(data, 0)

	// Serialize caveats
	for _, cav := range m.caveats {
		data = serializePacket(data, packet{
			fieldType: fieldIdentifier,
			data:      cav,
		})
		data = append(data, 0)
	}

	data = append(data, 0)

	// Serialize tail
	data = serializePacket(data, packet{
		fieldType: fieldSignature,
		data:      m.tail,
	})

	return data
}

// serializePacket converts packet to binary.
func serializePacket(data []byte, p packet) []byte {
	data = appendVarint(data, int(p.fieldType))
	data = appendVarint(data, len(p.data))
	data = append(data, p.data...)

	return data
}

func appendVarint(data []byte, x int) []byte {
	var buf [binary.MaxVarintLen32]byte
	n := binary.PutUvarint(buf[:], uint64(x))

	return append(data, buf[:n]...)
}

// ParseMacaroon converts binary to macaroon.
func ParseMacaroon(data []byte) (_ *Macaroon, err error) {
	if len(data) < 2 {
		return nil, errors.New("empty macaroon")
	}
	if data[0] != version {
		return nil, errors.New("invalid macaroon version")
	}
	// skip version
	data = data[1:]
	// Parse Location
	data, section, err := parseSection(data)
	if err != nil {
		return nil, err
	}
	if len(section) > 0 && section[0].fieldType == fieldLocation {
		section = section[1:]
	}
	if len(section) != 1 || section[0].fieldType != fieldIdentifier {
		return nil, errors.New("invalid macaroon header")
	}

	mac := Macaroon{}
	mac.head = section[0].data
	for {
		rest, section, err := parseSection(data)
		if err != nil {
			return nil, err
		}
		data = rest
		if len(section) == 0 {
			break
		}
		if len(section) > 0 && section[0].fieldType == fieldLocation {
			section = section[1:]
		}
		if len(section) == 0 || section[0].fieldType != fieldIdentifier {
			return nil, errors.New("no Identifier in caveat")
		}
		cav := append([]byte(nil), section[0].data...)
		section = section[1:]
		if len(section) == 0 {
			// First party caveat.
			// if cav.Location != "" {
			//     return nil, errors.New("location not allowed in first party caveat")
			// }
			mac.caveats = append(mac.caveats, cav)
			continue
		}
		if len(section) != 1 {
			return nil, errors.New("extra fields found in caveat")
		}
		if section[0].fieldType != fieldVerificationID {
			return nil, errors.New("invalid field found in caveat")
		}
		// cav.VerificationId = section[0].data
		mac.caveats = append(mac.caveats, cav)
	}
	_, sig, err := parsePacket(data)
	if err != nil {
		return nil, err
	}
	if sig.fieldType != fieldSignature {
		return nil, errors.New("unexpected field found instead of signature")
	}
	if len(sig.data) != 32 {
		return nil, errors.New("signature has unexpected length")
	}
	mac.tail = make([]byte, 32)
	copy(mac.tail, sig.data)
	// return data, nil
	//    Parse Identity
	//    Parse caveats
	//    Parse tail
	return &mac, nil
}

// parseSection returns data leftover and packet array.
func parseSection(data []byte) ([]byte, []packet, error) {
	prevFieldType := fieldType(-1)
	var packets []packet
	for {
		if len(data) == 0 {
			return nil, nil, errors.New("section extends past end of buffer")
		}
		rest, p, err := parsePacket(data)
		if err != nil {
			return nil, nil, err
		}
		if p.fieldType == fieldEOS {
			return rest, packets, nil
		}
		if p.fieldType <= prevFieldType {
			return nil, nil, errors.New("fields out of order")
		}
		packets = append(packets, p)
		prevFieldType = p.fieldType
		data = rest
	}
}

// parsePacket returns data leftover and packet.
func parsePacket(data []byte) ([]byte, packet, error) {
	data, ft, err := parseVarint(data)
	if err != nil {
		return nil, packet{}, err
	}

	p := packet{fieldType: fieldType(ft)}
	if p.fieldType == fieldEOS {
		return data, p, nil
	}
	data, packLen, err := parseVarint(data)
	if err != nil {
		return nil, packet{}, err
	}

	if packLen > len(data) {
		return nil, packet{}, errors.New("out of bounds")
	}
	if packLen == 0 {
		p.data = nil

		return data, p, nil
	}

	p.data = data[0:packLen]

	return data[packLen:], p, nil
}

func parseVarint(data []byte) ([]byte, int, error) {
	value, n := binary.Uvarint(data)
	if n <= 0 || value > 0x7fffffff {
		return nil, 0, errors.New("varint error")
	}
	return data[n:], int(value), nil
}
