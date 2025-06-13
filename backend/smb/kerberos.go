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

func resolveCcachePath(ccachePath string) (string, error) {
	if ccachePath == "" {
		ccachePath = os.Getenv("KRB5CCNAME")
	}

	switch {
	case strings.Contains(ccachePath, ":"):
		parts := strings.SplitN(ccachePath, ":", 2)
		prefix, path := parts[0], parts[1]
		switch prefix {
		case "FILE":
			return path, nil
		case "DIR":
			primary, err := os.ReadFile(filepath.Join(path, "primary"))
			if err != nil {
				return "", err
			}
			return filepath.Join(path, strings.TrimSpace(string(primary))), nil
		default:
			return "", fmt.Errorf("unsupported KRB5CCNAME: %s", ccachePath)
		}
	case ccachePath == "":
		u, err := user.Current()
		if err != nil {
			return "", err
		}
		return "/tmp/krb5cc_" + u.Uid, nil
	default:
		return ccachePath, nil
	}
}

func loadKerberosConfig() (*config.Config, error) {
	cfgPath := os.Getenv("KRB5_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/krb5.conf"
	}
	return config.Load(cfgPath)
}

// createKerberosClient creates a new Kerberos client.
func createKerberosClient(ccachePath string) (*client.Client, error) {
	ccachePath, err := resolveCcachePath(ccachePath)
	if err != nil {
		return nil, err
	}

	// check if we already have a client or an error for this ccache path
	if errVal, ok := kerberosErr.Load(ccachePath); ok {
		return nil, errVal.(error)
	}
	if clientVal, ok := kerberosClient.Load(ccachePath); ok {
		return clientVal.(*client.Client), nil
	}

	// create a new client if not found in the map
	cfg, err := loadKerberosConfig()
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
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
