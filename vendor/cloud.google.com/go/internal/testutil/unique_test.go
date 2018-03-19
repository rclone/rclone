// Copyright 2017 Google Inc. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package testutil

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	s := NewUIDSpace("prefix")
	tm := time.Date(2017, 1, 6, 0, 0, 0, 21, time.UTC)
	got := s.newID(tm)
	want := "prefix-20170106-21-0000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	s2 := NewUIDSpaceSep("prefix2", '_')
	got = s2.newID(tm)
	want = "prefix2_20170106_21_0000"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestTimestamp(t *testing.T) {
	s := NewUIDSpace("unique-ID")
	uid := s.New()
	got, ok := s.Timestamp(uid)
	if !ok {
		t.Fatal("got ok = false, want true")
	}
	if !startTime.Equal(got) {
		t.Errorf("got %s, want %s", got, startTime)
	}

	got, ok = s.Timestamp("unique-ID-20160308-123-8")
	if !ok {
		t.Fatal("got false, want true")
	}
	if want := time.Date(2016, 3, 8, 0, 0, 0, 123, time.UTC); !want.Equal(got) {
		t.Errorf("got %s, want %s", got, want)
	}
	if _, ok = s.Timestamp("invalid-time-1234"); ok {
		t.Error("got true, want false")
	}
}

func TestOlder(t *testing.T) {
	s := NewUIDSpace("uid")
	// A non-matching ID returns false.
	id2 := NewUIDSpace("different-prefix").New()
	if got, want := s.Older(id2, time.Second), false; got != want {
		t.Errorf("got %t, want %t", got, want)
	}
}
