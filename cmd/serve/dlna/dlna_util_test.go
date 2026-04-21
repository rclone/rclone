package dlna

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAdjustXML(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "no quotes",
			input:    []byte(`<title>Simple File</title>`),
			expected: `<title>Simple File</title>`,
		},
		{
			name:     "numeric quote entities",
			input:    []byte(`<title>File &#34;with quotes&#34; in name</title>`),
			expected: `<title>File &quot;with quotes&quot; in name</title>`,
		},
		{
			name:     "mixed entities",
			input:    []byte(`<title>File &#34;test&#34; &amp; &#34;demo&#34;</title>`),
			expected: `<title>File &quot;test&quot; &amp; &quot;demo&quot;</title>`,
		},
		{
			name:     "already correct entities",
			input:    []byte(`<title>File &quot;already correct&quot;</title>`),
			expected: `<title>File &quot;already correct&quot;</title>`,
		},
		{
			name:     "complex XML structure",
			input:    []byte(`<item><dc:title>Movie &#34;Title&#34;</dc:title><upnp:artist>Artist &#34;Name&#34;</upnp:artist></item>`),
			expected: `<item><dc:title>Movie &quot;Title&quot;</dc:title><upnp:artist>Artist &quot;Name&quot;</upnp:artist></item>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adjustXML(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}
