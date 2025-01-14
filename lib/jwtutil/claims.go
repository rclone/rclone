package jwtutil

import (
	"crypto/subtle"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v4"
)

// The following is the StandardClaims implementation from jwt-go v4,
// where it was marked as deprecated before removed in v5. Some small
// adjustments the original code have been made, to satisfy linters etc.
// The type has also been renamed to LegacyStandardClaims to avoid confusion.
// Source: https://github.com/golang-jwt/jwt/blob/v4/claims.go

// Copyright (c) 2012 Dave Grijalva
// Copyright (c) 2021 golang-jwt maintainers
//
// Permission is hereby granted, free of charge, to any person obtaining
// a copy of this software and associated documentation files (the
// "Software"), to deal in the Software without restriction, including
// without limitation the rights to use, copy, modify, merge, publish,
// distribute, sublicense, and/or sell copies of the Software, and to permit
// persons to whom the Software is furnished to do so, subject to the
// following conditions:
//
// The above copyright notice and this permission notice shall be included
// in all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL
// THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
// FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER
// DEALINGS IN THE SOFTWARE.

// LegacyStandardClaims are a structured version of the JWT Claims Set, as referenced at
// https://datatracker.ietf.org/doc/html/rfc7519#section-4. They do not follow the
// specification exactly, since they were based on an earlier draft of the
// specification and not updated. The main difference is that they only
// support integer-based date fields and singular audiences. This might lead to
// incompatibilities with other JWT implementations. The use of this is discouraged, instead
// the newer RegisteredClaims struct should be used.
type LegacyStandardClaims struct {
	Audience  string `json:"aud,omitempty"`
	ExpiresAt int64  `json:"exp,omitempty"`
	ID        string `json:"jti,omitempty"`
	IssuedAt  int64  `json:"iat,omitempty"`
	Issuer    string `json:"iss,omitempty"`
	NotBefore int64  `json:"nbf,omitempty"`
	Subject   string `json:"sub,omitempty"`
}

// Valid validates time based claims "exp, iat, nbf". There is no accounting for clock skew.
// As well, if any of the above claims are not in the token, it will still
// be considered a valid claim.
func (c LegacyStandardClaims) Valid() error {
	vErr := new(jwt.ValidationError)
	now := jwt.TimeFunc().Unix()

	// The claims below are optional, by default, so if they are set to the
	// default value in Go, let's not fail the verification for them.
	if !c.VerifyExpiresAt(now, false) {
		delta := time.Unix(now, 0).Sub(time.Unix(c.ExpiresAt, 0))
		vErr.Inner = fmt.Errorf("%s by %s", jwt.ErrTokenExpired, delta)
		vErr.Errors |= jwt.ValidationErrorExpired
	}

	if !c.VerifyIssuedAt(now, false) {
		vErr.Inner = jwt.ErrTokenUsedBeforeIssued
		vErr.Errors |= jwt.ValidationErrorIssuedAt
	}

	if !c.VerifyNotBefore(now, false) {
		vErr.Inner = jwt.ErrTokenNotValidYet
		vErr.Errors |= jwt.ValidationErrorNotValidYet
	}

	if vErr.Errors == 0 {
		return nil
	}

	return vErr
}

// VerifyAudience compares the aud claim against cmp.
// If required is false, this method will return true if the value matches or is unset
func (c *LegacyStandardClaims) VerifyAudience(cmp string, req bool) bool {
	return verifyAud([]string{c.Audience}, cmp, req)
}

// VerifyExpiresAt compares the exp claim against cmp (cmp < exp).
// If req is false, it will return true, if exp is unset.
func (c *LegacyStandardClaims) VerifyExpiresAt(cmp int64, req bool) bool {
	if c.ExpiresAt == 0 {
		return verifyExp(nil, time.Unix(cmp, 0), req)
	}

	t := time.Unix(c.ExpiresAt, 0)
	return verifyExp(&t, time.Unix(cmp, 0), req)
}

// VerifyIssuedAt compares the iat claim against cmp (cmp >= iat).
// If req is false, it will return true, if iat is unset.
func (c *LegacyStandardClaims) VerifyIssuedAt(cmp int64, req bool) bool {
	if c.IssuedAt == 0 {
		return verifyIat(nil, time.Unix(cmp, 0), req)
	}

	t := time.Unix(c.IssuedAt, 0)
	return verifyIat(&t, time.Unix(cmp, 0), req)
}

// VerifyNotBefore compares the nbf claim against cmp (cmp >= nbf).
// If req is false, it will return true, if nbf is unset.
func (c *LegacyStandardClaims) VerifyNotBefore(cmp int64, req bool) bool {
	if c.NotBefore == 0 {
		return verifyNbf(nil, time.Unix(cmp, 0), req)
	}

	t := time.Unix(c.NotBefore, 0)
	return verifyNbf(&t, time.Unix(cmp, 0), req)
}

// VerifyIssuer compares the iss claim against cmp.
// If required is false, this method will return true if the value matches or is unset
func (c *LegacyStandardClaims) VerifyIssuer(cmp string, req bool) bool {
	return verifyIss(c.Issuer, cmp, req)
}

// ----- helpers

func verifyAud(aud []string, cmp string, required bool) bool {
	if len(aud) == 0 {
		return !required
	}
	// use a var here to keep constant time compare when looping over a number of claims
	result := false

	var stringClaims string
	for _, a := range aud {
		if subtle.ConstantTimeCompare([]byte(a), []byte(cmp)) != 0 {
			result = true
		}
		stringClaims += a
	}

	// case where "" is sent in one or many aud claims
	if len(stringClaims) == 0 {
		return !required
	}

	return result
}

func verifyExp(exp *time.Time, now time.Time, required bool) bool {
	if exp == nil {
		return !required
	}
	return now.Before(*exp)
}

func verifyIat(iat *time.Time, now time.Time, required bool) bool {
	if iat == nil {
		return !required
	}
	return now.After(*iat) || now.Equal(*iat)
}

func verifyNbf(nbf *time.Time, now time.Time, required bool) bool {
	if nbf == nil {
		return !required
	}
	return now.After(*nbf) || now.Equal(*nbf)
}

func verifyIss(iss string, cmp string, required bool) bool {
	if iss == "" {
		return !required
	}
	return subtle.ConstantTimeCompare([]byte(iss), []byte(cmp)) != 0
}
