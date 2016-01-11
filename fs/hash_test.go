package fs_test

import (
	"bytes"
	"io"
	"testing"

	"github.com/ncw/rclone/fs"
)

func TestHashSet(t *testing.T) {
	var h fs.HashSet

	if h.Count() != 0 {
		t.Fatalf("expected empty set to have 0 elements, got %d", h.Count())
	}
	a := h.Array()
	if len(a) != 0 {
		t.Fatalf("expected empty slice, got %d", len(a))
	}

	h = h.Add(fs.HashMD5)
	if h.Count() != 1 {
		t.Fatalf("expected 1 element, got %d", h.Count())
	}
	if h.GetOne() != fs.HashMD5 {
		t.Fatalf("expected HashMD5, got %v", h.GetOne())
	}
	a = h.Array()
	if len(a) != 1 {
		t.Fatalf("expected 1 element, got %d", len(a))
	}
	if a[0] != fs.HashMD5 {
		t.Fatalf("expected HashMD5, got %v", a[0])
	}

	// Test overlap, with all hashes
	h = h.Overlap(fs.SupportedHashes)
	if h.Count() != 1 {
		t.Fatalf("expected 1 element, got %d", h.Count())
	}
	if h.GetOne() != fs.HashMD5 {
		t.Fatalf("expected HashMD5, got %v", h.GetOne())
	}
	if !h.SubsetOf(fs.SupportedHashes) {
		t.Fatalf("expected to be subset of all hashes")
	}
	if !h.SubsetOf(fs.NewHashSet(fs.HashMD5)) {
		t.Fatalf("expected to be subset of itself")
	}

	h = h.Add(fs.HashSHA1)
	if h.Count() != 2 {
		t.Fatalf("expected 2 elements, got %d", h.Count())
	}
	one := h.GetOne()
	if !(one == fs.HashMD5 || one == fs.HashSHA1) {
		t.Fatalf("expected to be either MD5 or SHA1, got %v", one)
	}
	if !h.SubsetOf(fs.SupportedHashes) {
		t.Fatalf("expected to be subset of all hashes")
	}
	if h.SubsetOf(fs.NewHashSet(fs.HashMD5)) {
		t.Fatalf("did not expect to be subset of only MD5")
	}
	if h.SubsetOf(fs.NewHashSet(fs.HashSHA1)) {
		t.Fatalf("did not expect to be subset of only SHA1")
	}
	if !h.SubsetOf(fs.NewHashSet(fs.HashMD5, fs.HashSHA1)) {
		t.Fatalf("expected to be subset of MD5/SHA1")
	}
	a = h.Array()
	if len(a) != 2 {
		t.Fatalf("expected 2 elements, got %d", len(a))
	}

	ol := h.Overlap(fs.NewHashSet(fs.HashMD5))
	if ol.Count() != 1 {
		t.Fatalf("expected 1 element overlap, got %d", ol.Count())
	}
	if !ol.Contains(fs.HashMD5) {
		t.Fatalf("expected overlap to be MD5, got %v", ol)
	}
	if ol.Contains(fs.HashSHA1) {
		t.Fatalf("expected overlap NOT to contain SHA1, got %v", ol)
	}

	ol = h.Overlap(fs.NewHashSet(fs.HashMD5, fs.HashSHA1))
	if ol.Count() != 2 {
		t.Fatalf("expected 2 element overlap, got %d", ol.Count())
	}
	if !ol.Contains(fs.HashMD5) {
		t.Fatalf("expected overlap to contain MD5, got %v", ol)
	}
	if !ol.Contains(fs.HashSHA1) {
		t.Fatalf("expected overlap to contain SHA1, got %v", ol)
	}
}

type hashTest struct {
	input  []byte
	output map[fs.HashType]string
}

var hashTestSet = []hashTest{
	hashTest{
		input: []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14},
		output: map[fs.HashType]string{
			fs.HashMD5:  "bf13fc19e5151ac57d4252e0e0f87abe",
			fs.HashSHA1: "3ab6543c08a75f292a5ecedac87ec41642d12166",
		},
	},
	// Empty data set
	hashTest{
		input: []byte{},
		output: map[fs.HashType]string{
			fs.HashMD5:  "d41d8cd98f00b204e9800998ecf8427e",
			fs.HashSHA1: "da39a3ee5e6b4b0d3255bfef95601890afd80709",
		},
	},
}

func TestMultiHasher(t *testing.T) {
	for _, test := range hashTestSet {
		mh := fs.NewMultiHasher()
		n, err := io.Copy(mh, bytes.NewBuffer(test.input))
		if err != nil {
			t.Fatal(err)
		}
		if int(n) != len(test.input) {
			t.Fatalf("copy mismatch: %d != %d", n, len(test.input))
		}
		sums := mh.Sums()
		for k, v := range sums {
			expect, ok := test.output[k]
			if !ok {
				t.Errorf("Unknown hash type %v, sum: %q", k, v)
			}
			if expect != v {
				t.Errorf("hash %v mismatch %q != %q", k, v, expect)
			}
		}
		// Test that all are present
		for k, v := range test.output {
			expect, ok := sums[k]
			if !ok {
				t.Errorf("did not calculate hash type %v, sum: %q", k, v)
			}
			if expect != v {
				t.Errorf("hash %d mismatch %q != %q", k, v, expect)
			}
		}
	}
}

func TestMultiHasherTypes(t *testing.T) {
	h := fs.HashSHA1
	for _, test := range hashTestSet {
		mh, err := fs.NewMultiHasherTypes(fs.NewHashSet(h))
		if err != nil {
			t.Fatal(err)
		}
		n, err := io.Copy(mh, bytes.NewBuffer(test.input))
		if err != nil {
			t.Fatal(err)
		}
		if int(n) != len(test.input) {
			t.Fatalf("copy mismatch: %d != %d", n, len(test.input))
		}
		sums := mh.Sums()
		if len(sums) != 1 {
			t.Fatalf("expected 1 sum, got %d", len(sums))
		}
		expect := test.output[h]
		if expect != sums[h] {
			t.Errorf("hash %v mismatch %q != %q", h, sums[h], expect)
		}
	}
}

func TestHashStream(t *testing.T) {
	for _, test := range hashTestSet {
		sums, err := fs.HashStream(bytes.NewBuffer(test.input))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range sums {
			expect, ok := test.output[k]
			if !ok {
				t.Errorf("Unknown hash type %v, sum: %q", k, v)
			}
			if expect != v {
				t.Errorf("hash %v mismatch %q != %q", k, v, expect)
			}
		}
		// Test that all are present
		for k, v := range test.output {
			expect, ok := sums[k]
			if !ok {
				t.Errorf("did not calculate hash type %v, sum: %q", k, v)
			}
			if expect != v {
				t.Errorf("hash %v mismatch %q != %q", k, v, expect)
			}
		}
	}
}

func TestHashStreamTypes(t *testing.T) {
	h := fs.HashSHA1
	for _, test := range hashTestSet {
		sums, err := fs.HashStreamTypes(bytes.NewBuffer(test.input), fs.NewHashSet(h))
		if err != nil {
			t.Fatal(err)
		}
		if len(sums) != 1 {
			t.Fatalf("expected 1 sum, got %d", len(sums))
		}
		expect := test.output[h]
		if expect != sums[h] {
			t.Errorf("hash %d mismatch %q != %q", h, sums[h], expect)
		}
	}
}

func TestHashSetStringer(t *testing.T) {
	h := fs.NewHashSet(fs.HashSHA1, fs.HashMD5)
	s := h.String()
	expect := "[MD5, SHA-1]"
	if s != expect {
		t.Errorf("unexpected stringer: was %q, expected %q", s, expect)
	}
	h = fs.NewHashSet(fs.HashSHA1)
	s = h.String()
	expect = "[SHA-1]"
	if s != expect {
		t.Errorf("unexpected stringer: was %q, expected %q", s, expect)
	}
	h = fs.NewHashSet()
	s = h.String()
	expect = "[]"
	if s != expect {
		t.Errorf("unexpected stringer: was %q, expected %q", s, expect)
	}
}

func TestHashStringer(t *testing.T) {
	h := fs.HashMD5
	s := h.String()
	expect := "MD5"
	if s != expect {
		t.Errorf("unexpected stringer: was %q, expected %q", s, expect)
	}
	h = fs.HashNone
	s = h.String()
	expect = "None"
	if s != expect {
		t.Errorf("unexpected stringer: was %q, expected %q", s, expect)
	}
}
