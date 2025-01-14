package smb

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
)

var (
	kerberosClient *client.Client
	kerberosErr    error
	kerberosOnce   sync.Once
)

// getKerberosClient returns a Kerberos client that can be used to authenticate.
func getKerberosClient() (*client.Client, error) {
	if kerberosClient == nil || kerberosErr == nil {
		kerberosOnce.Do(func() {
			kerberosClient, kerberosErr = createKerberosClient()
		})
	}

	return kerberosClient, kerberosErr
}

// createKerberosClient creates a new Kerberos client.
func createKerberosClient() (*client.Client, error) {
	cfgPath := os.Getenv("KRB5_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/krb5.conf"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	// Determine the ccache location from the environment, falling back to the
	// default location.
	ccachePath := os.Getenv("KRB5CCNAME")
	switch {
	case strings.Contains(ccachePath, ":"):
		parts := strings.SplitN(ccachePath, ":", 2)
		switch parts[0] {
		case "FILE":
			ccachePath = parts[1]
		case "DIR":
			primary, err := os.ReadFile(filepath.Join(parts[1], "primary"))
			if err != nil {
				return nil, err
			}
			ccachePath = filepath.Join(parts[1], strings.TrimSpace(string(primary)))
		default:
			return nil, fmt.Errorf("unsupported KRB5CCNAME: %s", ccachePath)
		}
	case ccachePath == "":
		u, err := user.Current()
		if err != nil {
			return nil, err
		}

		ccachePath = "/tmp/krb5cc_" + u.Uid
	}

	ccache, err := credentials.LoadCCache(ccachePath)
	if err != nil {
		return nil, err
	}

	return client.NewFromCCache(ccache, cfg)
}
