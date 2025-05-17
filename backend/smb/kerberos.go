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
	kerberosClient sync.Map // map[string]*client.Client
	kerberosErr    sync.Map // map[string]error
)

// createKerberosClient creates a new Kerberos client.
func createKerberosClient(ccachePath string) (*client.Client, error) {
	cfgPath := os.Getenv("KRB5_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/krb5.conf"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	// If ccachePath is empty, use the global default ccache location.
	if ccachePath == "" {
		ccachePath = os.Getenv("KRB5CCNAME")
	}
	// Determine the ccache location, falling back to the
	// default location.
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

	if errVal, ok := kerberosErr.Load(ccachePath); ok {
		return nil, errVal.(error)
	}
	if clientVal, ok := kerberosClient.Load(ccachePath); ok {
		return clientVal.(*client.Client), nil
	}

	ccache, err := credentials.LoadCCache(ccachePath)
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
	}
	cl, err := client.NewFromCCache(ccache, cfg)
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
	}
	kerberosClient.Store(ccachePath, cl)
	return cl, nil
}
