// Copyright 2016 Google Inc. All Rights Reserved.
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

package main

import (
	"testing"
	"time"

	"cloud.google.com/go/bigtable"
	"github.com/google/go-cmp/cmp"
)

func TestParseDuration(t *testing.T) {
	tests := []struct {
		in string
		// out or fail are mutually exclusive
		out  time.Duration
		fail bool
	}{
		{in: "10ms", out: 10 * time.Millisecond},
		{in: "3s", out: 3 * time.Second},
		{in: "60m", out: 60 * time.Minute},
		{in: "12h", out: 12 * time.Hour},
		{in: "7d", out: 168 * time.Hour},

		{in: "", fail: true},
		{in: "0", fail: true},
		{in: "7ns", fail: true},
		{in: "14mo", fail: true},
		{in: "3.5h", fail: true},
		{in: "106752d", fail: true}, // overflow
	}
	for _, tc := range tests {
		got, err := parseDuration(tc.in)
		if !tc.fail && err != nil {
			t.Errorf("parseDuration(%q) unexpectedly failed: %v", tc.in, err)
			continue
		}
		if tc.fail && err == nil {
			t.Errorf("parseDuration(%q) did not fail", tc.in)
			continue
		}
		if tc.fail {
			continue
		}
		if got != tc.out {
			t.Errorf("parseDuration(%q) = %v, want %v", tc.in, got, tc.out)
		}
	}
}

func TestParseGCPolicy(t *testing.T) {
	tests := []struct {
		in   string
		out  bigtable.GCPolicy
		fail bool
	}{
		{in: "maxage=1h", out: bigtable.MaxAgePolicy(time.Hour * 1)},
		{in: "maxversions=2", out: bigtable.MaxVersionsPolicy(int(2))},
		{in: "maxversions=2 and maxage=1h", out: bigtable.IntersectionPolicy([]bigtable.GCPolicy{bigtable.MaxVersionsPolicy(int(2)), bigtable.MaxAgePolicy(time.Hour * 1)}...)},
		{in: "maxversions=2 or maxage=1h", out: bigtable.UnionPolicy([]bigtable.GCPolicy{bigtable.MaxVersionsPolicy(int(2)), bigtable.MaxAgePolicy(time.Hour * 1)}...)},

		{in: "maxage=1", fail: true},
		{in: "maxage = 1h", fail: true},
		{in: "maxage =1h", fail: true},
		{in: "maxage= 1h", fail: true},
		{in: "foomaxage=1h", fail: true},
		{in: "maxversions=1h", fail: true},
		{in: "maxversions= 1", fail: true},
		{in: "maxversions = 1", fail: true},
		{in: "maxversions =1", fail: true},
		{in: "barmaxversions=1", fail: true},
		{in: "maxage = 1h or maxversions=1h", fail: true},
		{in: "foomaxversions=2 or maxage=1h", fail: true},
		{in: "maxversions=2 or barmaxage=1h", fail: true},
		{in: "foomaxversions=2 or barmaxage=1h", fail: true},
		{in: "maxage = 1h and maxversions=1h", fail: true},
		{in: "foomaxage=1h and maxversions=1", fail: true},
		{in: "maxage=1h and barmaxversions=1", fail: true},
		{in: "foomaxage=1h and barmaxversions=1", fail: true},
	}
	for _, tc := range tests {
		got, err := parseGCPolicy(tc.in)
		if !tc.fail && err != nil {
			t.Errorf("parseGCPolicy(%q) unexpectedly failed: %v", tc.in, err)
			continue
		}
		if tc.fail && err == nil {
			t.Errorf("parseGCPolicy(%q) did not fail", tc.in)
			continue
		}
		if tc.fail {
			continue
		}
		var cmpOpts cmp.Options
		cmpOpts = append(cmpOpts, cmp.AllowUnexported(bigtable.IntersectionPolicy([]bigtable.GCPolicy{}...)), cmp.AllowUnexported(bigtable.UnionPolicy([]bigtable.GCPolicy{}...)))
		if !cmp.Equal(got, tc.out, cmpOpts) {
			t.Errorf("parseGCPolicy(%q) =%v, want %v", tc.in, got, tc.out)
		}
	}
}
