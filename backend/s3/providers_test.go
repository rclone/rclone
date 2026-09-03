package s3

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProviderSignAcceptEncodingQuirks(t *testing.T) {
	// Providers that mutate or proxy Accept-Encoding must not sign it by default.
	for _, name := range []string{"Ceph", "Linode", "GCS"} {
		p := loadProvider(name)
		require.NotNil(t, p, name)
		require.NotNil(t, p.Quirks.SignAcceptEncoding, "%s should set sign_accept_encoding", name)
		assert.False(t, *p.Quirks.SignAcceptEncoding, "%s sign_accept_encoding", name)
	}

	// AWS keeps the default (unset quirk => sign Accept-Encoding).
	aws := loadProvider("AWS")
	require.NotNil(t, aws)
	assert.Nil(t, aws.Quirks.SignAcceptEncoding)
}
