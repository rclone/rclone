// Copyright (c) Dropbox, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package secondary_emails : has no documentation (yet)
package secondary_emails

// SecondaryEmail : has no documentation (yet)
type SecondaryEmail struct {
	// Email : Secondary email address.
	Email string `json:"email"`
	// IsVerified : Whether or not the secondary email address is verified to be
	// owned by a user.
	IsVerified bool `json:"is_verified"`
}

// NewSecondaryEmail returns a new SecondaryEmail instance
func NewSecondaryEmail(Email string, IsVerified bool) *SecondaryEmail {
	s := new(SecondaryEmail)
	s.Email = Email
	s.IsVerified = IsVerified
	return s
}
