package jwtutil

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// The following is the declaration of the StandardClaims type from jwt-go v4
// (https://github.com/golang-jwt/jwt/blob/v4/claims.go), where it was marked
// as deprecated before later removed in v5. It was distributed under the terms
// of the MIT License (https://github.com/golang-jwt/jwt/blob/v4/LICENSE), with
// the copy right notice included below. We have renamed the type to
// LegacyStandardClaims to avoid confusion, and made it compatible with
// jwt-go v5 by implementing functions to satisfy the changed Claims interface.

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

// GetExpirationTime implements the Claims interface.
func (c LegacyStandardClaims) GetExpirationTime() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.ExpiresAt, 0)), nil
}

// GetIssuedAt implements the Claims interface.
func (c LegacyStandardClaims) GetIssuedAt() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.IssuedAt, 0)), nil
}

// GetNotBefore implements the Claims interface.
func (c LegacyStandardClaims) GetNotBefore() (*jwt.NumericDate, error) {
	return jwt.NewNumericDate(time.Unix(c.NotBefore, 0)), nil
}

// GetIssuer implements the Claims interface.
func (c LegacyStandardClaims) GetIssuer() (string, error) {
	return c.Issuer, nil
}

// GetSubject implements the Claims interface.
func (c LegacyStandardClaims) GetSubject() (string, error) {
	return c.Subject, nil
}

// GetAudience implements the Claims interface.
func (c LegacyStandardClaims) GetAudience() (jwt.ClaimStrings, error) {
	return []string{c.Audience}, nil
}
