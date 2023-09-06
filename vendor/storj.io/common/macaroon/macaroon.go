// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package macaroon

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
)

// Macaroon is a struct that determine contextual caveats and authorization.
type Macaroon struct {
	head    []byte
	caveats [][]byte
	tail    []byte
}

// NewUnrestricted creates Macaroon with random Head and generated Tail.
func NewUnrestricted(secret []byte) (*Macaroon, error) {
	head, err := NewSecret()
	if err != nil {
		return nil, err
	}

	return NewUnrestrictedFromParts(head, secret), nil
}

// NewUnrestrictedFromParts constructs an unrestricted Macaroon from the provided head and secret.
func NewUnrestrictedFromParts(head, secret []byte) *Macaroon {
	return &Macaroon{
		head: head,
		tail: sign(secret, head),
	}
}

func sign(secret []byte, data []byte) []byte {
	signer := hmac.New(sha256.New, secret)
	_, err := signer.Write(data)
	if err != nil {
		// Error skipped because sha256 does not return error
		panic(err)
	}

	return signer.Sum(nil)
}

// NewSecret generates cryptographically random 32 bytes.
func NewSecret() (secret []byte, err error) {
	secret = make([]byte, 32)

	_, err = rand.Read(secret)
	if err != nil {
		return nil, err
	}

	return secret, nil
}

// AddFirstPartyCaveat creates signed macaroon with appended caveat.
func (m *Macaroon) AddFirstPartyCaveat(c []byte) (macaroon *Macaroon, err error) {
	macaroon = m.Copy()

	macaroon.caveats = append(macaroon.caveats, c)
	macaroon.tail = sign(macaroon.tail, c)

	return macaroon, nil
}

// Validate reconstructs with all caveats from the secret and compares tails,
// returning true if the tails match.
func (m *Macaroon) Validate(secret []byte) (ok bool) {
	tail := sign(secret, m.head)
	for _, cav := range m.caveats {
		tail = sign(tail, cav)
	}

	return subtle.ConstantTimeCompare(tail, m.tail) == 1
}

// Tails returns all ancestor tails up to and including the current tail.
func (m *Macaroon) Tails(secret []byte) [][]byte {
	tails := make([][]byte, 0, len(m.caveats)+1)
	tail := sign(secret, m.head)
	tails = append(tails, tail)
	for _, cav := range m.caveats {
		tail = sign(tail, cav)
		tails = append(tails, tail)
	}
	return tails
}

// ValidateAndTails combines Validate and Tails to a single method.
func (m *Macaroon) ValidateAndTails(secret []byte) (bool, [][]byte) {
	tails := make([][]byte, 0, len(m.caveats)+1)
	tail := sign(secret, m.head)
	tails = append(tails, tail)
	for _, cav := range m.caveats {
		tail = sign(tail, cav)
		tails = append(tails, tail)
	}
	return subtle.ConstantTimeCompare(tail, m.tail) == 1, tails
}

// Head returns copy of macaroon head.
func (m *Macaroon) Head() (head []byte) {
	if len(m.head) == 0 {
		return nil
	}
	return append([]byte(nil), m.head...)
}

// CaveatLen returns the number of caveats this macaroon has.
func (m *Macaroon) CaveatLen() int {
	return len(m.caveats)
}

// Caveats returns copy of macaroon caveats.
func (m *Macaroon) Caveats() (caveats [][]byte) {
	if len(m.caveats) == 0 {
		return nil
	}
	caveats = make([][]byte, 0, len(m.caveats))
	for _, cav := range m.caveats {
		caveats = append(caveats, append([]byte(nil), cav...))
	}
	return caveats
}

// Tail returns copy of macaroon tail.
func (m *Macaroon) Tail() (tail []byte) {
	if len(m.tail) == 0 {
		return nil
	}
	return append([]byte(nil), m.tail...)
}

// Copy return copy of macaroon.
func (m *Macaroon) Copy() *Macaroon {
	return &Macaroon{
		head:    m.Head(),
		caveats: m.Caveats(),
		tail:    m.Tail(),
	}
}
