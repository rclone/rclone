package fs

import (
	"bytes"
	"reflect"
	"testing"
)

func TestSizeSuffixString(t *testing.T) {
	for _, test := range []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{102, "0.100k"},
		{1024, "1k"},
		{1024 * 1024, "1M"},
		{1024 * 1024 * 1024, "1G"},
		{10 * 1024 * 1024 * 1024, "10G"},
		{10.1 * 1024 * 1024 * 1024, "10.100G"},
	} {
		ss := SizeSuffix(test.in)
		got := ss.String()
		if test.want != got {
			t.Errorf("Want %v got %v", test.want, got)
		}
	}
}

func TestSizeSuffixSet(t *testing.T) {
	for i, test := range []struct {
		in   string
		want int64
		err  bool
	}{
		{"0", 0, false},
		{"0.1k", 102, false},
		{"0.1", 102, false},
		{"1K", 1024, false},
		{"1", 1024, false},
		{"2.5", 1024 * 2.5, false},
		{"1M", 1024 * 1024, false},
		{"1.g", 1024 * 1024 * 1024, false},
		{"10G", 10 * 1024 * 1024 * 1024, false},
		{"", 0, true},
		{"1p", 0, true},
		{"1.p", 0, true},
		{"1p", 0, true},
		{"-1K", 0, true},
	} {
		ss := SizeSuffix(0)
		err := ss.Set(test.in)
		if (err != nil) != test.err {
			t.Errorf("%d: Expecting error %v but got error %v", i, test.err, err)
		}
		got := int64(ss)
		if test.want != got {
			t.Errorf("%d: Want %v got %v", i, test.want, got)
		}
	}
}

func TestReveal(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{"2sTcyNrA", "potato"},
	} {
		got := Reveal(test.in)
		if got != test.want {
			t.Errorf("%q: want %q got %q", test.in, test.want, got)
		}
		if Obscure(got) != test.in {
			t.Errorf("%q: wasn't bidirectional", test.in)
		}
	}
}

func TestConfigLoad(t *testing.T) {
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/plain.conf"
	defer func() {
		ConfigPath = oldConfigPath
	}()
	configKey = nil // reset password
	c, err := loadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	sections := c.GetSectionList()
	var expect = []string{"RCLONE_ENCRYPT_V0", "nounc", "unc"}
	if !reflect.DeepEqual(sections, expect) {
		t.Fatalf("%v != %v", sections, expect)
	}

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	if !reflect.DeepEqual(keys, expect) {
		t.Fatalf("%v != %v", keys, expect)
	}
}

func TestConfigLoadEncrypted(t *testing.T) {
	var err error
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/encrypted.conf"
	defer func() {
		ConfigPath = oldConfigPath
		configKey = nil // reset password
	}()

	// Set correct password
	err = setPassword("asdf")
	if err != nil {
		t.Fatal(err)
	}
	c, err := loadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	sections := c.GetSectionList()
	var expect = []string{"nounc", "unc"}
	if !reflect.DeepEqual(sections, expect) {
		t.Fatalf("%v != %v", sections, expect)
	}

	keys := c.GetKeyList("nounc")
	expect = []string{"type", "nounc"}
	if !reflect.DeepEqual(keys, expect) {
		t.Fatalf("%v != %v", keys, expect)
	}
}

func TestConfigLoadEncryptedFailures(t *testing.T) {
	var err error

	// This file should be too short to be decoded.
	oldConfigPath := ConfigPath
	ConfigPath = "./testdata/enc-short.conf"
	defer func() { ConfigPath = oldConfigPath }()
	_, err = loadConfigFile()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log("Correctly got:", err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-invalid.conf"
	_, err = loadConfigFile()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log("Correctly got:", err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/enc-too-new.conf"
	_, err = loadConfigFile()
	if err == nil {
		t.Fatal("expected error")
	}
	t.Log("Correctly got:", err)

	// This file contains invalid base64 characters.
	ConfigPath = "./testdata/filenotfound.conf"
	c, err := loadConfigFile()
	if err != nil {
		t.Fatal(err)
	}
	if len(c.GetSectionList()) != 0 {
		t.Fatalf("Expected 0-length section, got %d entries", len(c.GetSectionList()))
	}
}

func TestPassword(t *testing.T) {
	defer func() {
		configKey = nil // reset password
	}()
	var err error
	// Empty password should give error
	err = setPassword("  \t  ")
	if err == nil {
		t.Fatal("expected error")
	}

	// Test invalid utf8 sequence
	err = setPassword(string([]byte{0xff, 0xfe, 0xfd}) + "abc")
	if err == nil {
		t.Fatal("expected error")
	}

	// Simple check of wrong passwords
	hashedKeyCompare(t, "mis", "match", false)

	// Check that passwords match with trimmed whitespace
	hashedKeyCompare(t, "   abcdef   \t", "abcdef", true)

	// Check that passwords match after unicode normalization
	hashedKeyCompare(t, "ﬀ\u0041\u030A", "ffÅ", true)

	// Check that passwords preserves case
	hashedKeyCompare(t, "abcdef", "ABCDEF", false)

}

func hashedKeyCompare(t *testing.T, a, b string, shouldMatch bool) {
	err := setPassword(a)
	if err != nil {
		t.Fatal(err)
	}
	k1 := configKey

	err = setPassword(b)
	if err != nil {
		t.Fatal(err)
	}
	k2 := configKey
	matches := bytes.Equal(k1, k2)
	if shouldMatch && !matches {
		t.Fatalf("%v != %v", k1, k2)
	}
	if !shouldMatch && matches {
		t.Fatalf("%v == %v", k1, k2)
	}
}
