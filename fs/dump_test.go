package fs

import (
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Check it satisfies the interface
var _ pflag.Value = (*DumpFlags)(nil)

func TestDumpFlagsString(t *testing.T) {
	assert.Equal(t, "", DumpFlags(0).String())
	assert.Equal(t, "headers", (DumpHeaders).String())
	assert.Equal(t, "headers,bodies", (DumpHeaders | DumpBodies).String())
	assert.Equal(t, "headers,bodies,requests,responses,auth,filters", (DumpHeaders | DumpBodies | DumpRequests | DumpResponses | DumpAuth | DumpFilters).String())
	assert.Equal(t, "headers,Unknown-0x8000", (DumpHeaders | DumpFlags(0x8000)).String())
}

func TestDumpFlagsSet(t *testing.T) {
	for _, test := range []struct {
		in      string
		want    DumpFlags
		wantErr string
	}{
		{"", DumpFlags(0), ""},
		{"bodies", DumpBodies, ""},
		{"bodies,headers,auth", DumpBodies | DumpHeaders | DumpAuth, ""},
		{"bodies,headers,auth", DumpBodies | DumpHeaders | DumpAuth, ""},
		{"headers,bodies,requests,responses,auth,filters", DumpHeaders | DumpBodies | DumpRequests | DumpResponses | DumpAuth | DumpFilters, ""},
		{"headers,bodies,unknown,auth", 0, "Unknown dump flag \"unknown\""},
	} {
		f := DumpFlags(-1)
		initial := f
		err := f.Set(test.in)
		if err != nil {
			if test.wantErr == "" {
				t.Errorf("Got an error when not expecting one on %q: %v", test.in, err)
			} else {
				assert.Contains(t, err.Error(), test.wantErr)
			}
			assert.Equal(t, initial, f, test.want)
		} else {
			if test.wantErr != "" {
				t.Errorf("Got no error when expecting one on %q", test.in)
			} else {
				assert.Equal(t, test.want, f)
			}
		}

	}
}

func TestDumpFlagsType(t *testing.T) {
	f := DumpFlags(0)
	assert.Equal(t, "DumpFlags", f.Type())
}
