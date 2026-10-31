//go:build !plan9 && !solaris

package iclouddrive

import (
	"testing"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigAuthSessionKeepsRegion(t *testing.T) {
	m := configmap.Simple{}
	s := api.NewSession()
	require.NoError(t, s.SetDomainToUse("iCloud.com.cn"))
	saveAuthSession(m, s, nil)
	client, _, err := resumeConfigClient(m, "", "", "", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "iCloud.com.cn", client.Session.DomainToUse)
	saveAuthCredentials(m, client, t.Name())
	assert.Equal(t, regionChina, m[configRegion])
	assert.Empty(t, m[configAuthSession])
}

func TestConfigRegion(t *testing.T) {
	for _, tc := range []struct {
		region, domain, savedRegion string
	}{
		{"", "", ""},
		{regionGlobal, "", ""},
		{regionChina, "iCloud.com.cn", regionChina},
	} {
		t.Run(tc.region, func(t *testing.T) {
			m := configmap.Simple{}
			s := api.NewSession()
			require.NoError(t, setSessionRegion(s, tc.region))
			assert.Equal(t, tc.domain, s.DomainToUse)
			saveSessionRegion(m, s)
			if tc.savedRegion == "" {
				assert.NotContains(t, m, configRegion)
			} else {
				assert.Equal(t, tc.savedRegion, m[configRegion])
			}
		})
	}
	require.Error(t, setSessionRegion(api.NewSession(), "invalid"))
}

func TestGlobalAuthDoesNotSaveRegion(t *testing.T) {
	m := configmap.Simple{}
	client, err := api.New("", "", "", "", nil, nil, t.Name(), "")
	require.NoError(t, err)
	saveAuthCredentials(m, client, t.Name())
	assert.NotContains(t, m, configRegion)
}
