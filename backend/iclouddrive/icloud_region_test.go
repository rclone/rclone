//go:build !plan9 && !solaris

package iclouddrive

import (
	"testing"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigAuthSessionKeepsDomain(t *testing.T) {
	m := configmap.Simple{}
	s := api.NewSession()
	require.NoError(t, s.SetDomainToUse("iCloud.com.cn"))
	saveAuthSession(m, s, nil)
	client, _, err := resumeConfigClient(m, "", "", "", "", nil)
	require.NoError(t, err)
	assert.Equal(t, "iCloud.com.cn", client.Session.DomainToUse)
	saveAuthCredentials(m, client, t.Name())
	assert.Equal(t, "iCloud.com.cn", m[configDomain])
	assert.Empty(t, m[configAuthSession])
}
