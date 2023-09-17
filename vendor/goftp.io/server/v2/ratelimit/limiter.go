// Copyright 2020 The goftp Authors. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package ratelimit

import (
	"time"
)

// Limiter represents a rate limiter
type Limiter struct {
	rate  time.Duration
	count int64
	t     time.Time
}

// New create a limiter for transfer speed, parameter rate means bytes per second
// 0 means don't limit
func New(rate int64) *Limiter {
	return &Limiter{
		rate:  time.Duration(rate),
		count: 0,
		t:     time.Now(),
	}
}

// Wait sleep when write count bytes
func (l *Limiter) Wait(count int) {
	if l.rate == 0 {
		return
	}
	l.count += int64(count)
	t := time.Duration(l.count)*time.Second/l.rate - time.Since(l.t)
	if t > 0 {
		time.Sleep(t)
	}
}
