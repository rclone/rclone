// Tests for swift metadata
package swift

import (
	"testing"
	"time"
)

func TestHeadersToMetadata(t *testing.T) {
}

func TestHeadersToAccountMetadata(t *testing.T) {
}

func TestHeadersToContainerMetadata(t *testing.T) {
}

func TestHeadersToObjectMetadata(t *testing.T) {
}

func TestMetadataToHeaders(t *testing.T) {
}

func TestMetadataToAccountHeaders(t *testing.T) {
}

func TestMetadataToContainerHeaders(t *testing.T) {
}

func TestMetadataToObjectHeaders(t *testing.T) {
}

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
		{0, ".0"},
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

func TestGetModTime(t *testing.T) {
	for _, d := range []struct {
		ns string
		t  string
	}{
		{"1354040105", "2012-11-27T18:15:05Z"},
		{"1354040105.", "2012-11-27T18:15:05Z"},
		{"1354040105.0", "2012-11-27T18:15:05Z"},
		{"1354040105.000000000000", "2012-11-27T18:15:05Z"},
		{"1354040105.123", "2012-11-27T18:15:05.123Z"},
		{"1354040105.123456", "2012-11-27T18:15:05.123456Z"},
		{"1354040105.123456789", "2012-11-27T18:15:05.123456789Z"},
		{"1354040105.123456789123", "2012-11-27T18:15:05.123456789Z"},
		{"0", "1970-01-01T00:00:00.000000000Z"},
	} {
		expected, err := time.Parse(time.RFC3339, d.t)
		if err != nil {
			t.Error("Bad test", err)
		}
		m := Metadata{"mtime": d.ns}
		actual, err := m.GetModTime()
		if err != nil {
			t.Error("Parse error", err)
		}
		if !actual.Equal(expected) {
			t.Error("Expecting", expected, expected.UnixNano(), "got", actual, actual.UnixNano())
		}
	}
	for _, ns := range []string{
		"EMPTY",
		"",
		" 1",
		"- 1",
		"- 1",
		"1.-1",
		"1.0.0",
		"1x0",
	} {
		m := Metadata{}
		if ns != "EMPTY" {
			m["mtime"] = ns
		}
		actual, err := m.GetModTime()
		if err == nil {
			t.Error("Expected error not produced")
		}
		if !actual.IsZero() {
			t.Error("Expected output to be zero")
		}
	}
}

func TestSetModTime(t *testing.T) {
	for _, d := range []struct {
		ns string
		t  string
	}{
		{"1354040105", "2012-11-27T18:15:05Z"},
		{"1354040105", "2012-11-27T18:15:05.000000Z"},
		{"1354040105.123", "2012-11-27T18:15:05.123Z"},
		{"1354040105.123456", "2012-11-27T18:15:05.123456Z"},
		{"1354040105.123456789", "2012-11-27T18:15:05.123456789Z"},
		{"0", "1970-01-01T00:00:00.000000000Z"},
	} {
		time, err := time.Parse(time.RFC3339, d.t)
		if err != nil {
			t.Error("Bad test", err)
		}
		m := Metadata{}
		m.SetModTime(time)
		if m["mtime"] != d.ns {
			t.Error("mtime wrong", m, "should be", d.ns)
		}
	}
}
