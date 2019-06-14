// Copyright 2010 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package json

import "testing"

var scanTests = []struct {
	data   string
	length int
	ok     bool
}{
	{`foo`, 2, false},
	{`}{`, 1, false},
	{`{]`, 2, false},
	{`{}`, 2, true},
	{`{"foo":"bar"}`, 13, true},
	{`{"foo":"bar","bar":{"baz":["qux"]}`, 34, false},
	{`{"foo":"bar","bar":{"baz":["qux"]}}`, 35, true},
}

func TestScan(t *testing.T) {
	for _, st := range scanTests {
		scanned, err := Scan([]byte(st.data))
		if scanned != st.length {
			t.Errorf("Scan length error: expected: %d; got: %d; input: %s",
				st.length, scanned, st.data)
		}

		if err != nil && st.ok {
			t.Errorf("Scan failed with err: %s; input: %s", err, st.data)
		}

		if err == nil && !st.ok {
			t.Errorf("Scan should fail for input: %s", st.data)
		}
	}
}
