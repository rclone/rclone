// Tests for swiftsync
package main

import (
	"testing"
)

func TestNsToFloatString(t *testing.T) {
	for _, d := range []struct {
		ns int64
		fs string
	}{
		{0, "0"},
		{1, "0.000000001"},
		{1000, "0.000001"},
		{1000000, "0.001"},
		{100000000, "0.1"},
		{1000000000, "1"},
		{10000000000, "10"},
		{12345678912, "12.345678912"},
		{12345678910, "12.34567891"},
		{12345678900, "12.3456789"},
		{12345678000, "12.345678"},
		{12345670000, "12.34567"},
		{12345600000, "12.3456"},
		{12345000000, "12.345"},
		{12340000000, "12.34"},
		{12300000000, "12.3"},
		{12000000000, "12"},
		{10000000000, "10"},
		{1347717491123123123, "1347717491.123123123"},
	} {
		if nsToFloatString(d.ns) != d.fs {
			t.Error("Failed", d.ns, "!=", d.fs)
		}
		if d.ns > 0 && nsToFloatString(-d.ns) != "-"+d.fs {
			t.Error("Failed on negative", d.ns, "!=", d.fs)
		}
	}
}

func TestFloatStringToNs(t *testing.T) {
	for _, d := range []struct {
		ns int64
		fs string
	}{
		{0, "0"},
		{0, "0."},
		{0, "0.0"},
		{0, "0.0000000001"},
		{1, "0.000000001"},
		{1000, "0.000001"},
		{1000000, "0.001"},
		{100000000, "0.1"},
		{100000000, "0.10"},
		{100000000, "0.1000000001"},
		{1000000000, "1"},
		{1000000000, "1."},
		{1000000000, "1.0"},
		{10000000000, "10"},
		{12345678912, "12.345678912"},
		{12345678912, "12.3456789129"},
		{12345678912, "12.34567891299"},
		{12345678910, "12.34567891"},
		{12345678900, "12.3456789"},
		{12345678000, "12.345678"},
		{12345670000, "12.34567"},
		{12345600000, "12.3456"},
		{12345000000, "12.345"},
		{12340000000, "12.34"},
		{12300000000, "12.3"},
		{12000000000, "12"},
		{10000000000, "10"},
		// This is a typical value which has more bits in than a float64
		{1347717491123123123, "1347717491.123123123"},
	} {
		ns, err := floatStringToNs(d.fs)
		if err != nil {
			t.Error("Failed conversion", err)
		}
		if ns != d.ns {
			t.Error("Failed", d.fs, "!=", d.ns, "was", ns)
		}
		if d.ns > 0 {
			ns, err := floatStringToNs("-" + d.fs)
			if err != nil {
				t.Error("Failed conversion", err)
			}
			if ns != -d.ns {
				t.Error("Failed on negative", -d.ns, "!=", "-"+d.fs)
			}
		}
	}

	// These are expected to produce errors
	for _, fs := range []string{
		"",
		".0",
		" 1",
		"- 1",
		"- 1",
		"1.-1",
		"1.0.0",
		"1x0",
	} {
		ns, err := floatStringToNs(fs)
		if err == nil {
			t.Error("Didn't produce expected error", fs, ns)
		}
	}

}
