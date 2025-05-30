package smb

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jcmturner/gokrb5/v8/client"
	"github.com/jcmturner/gokrb5/v8/config"
	"github.com/jcmturner/gokrb5/v8/credentials"
)

var (
	// kerberosClient caches Kerberos clients keyed by resolved ccache path.
	// Clients are reused unless the associated ccache file changes.
	kerberosClient sync.Map // map[string]*client.Client

	// kerberosErr caches errors encountered when loading Kerberos clients.
	// Prevents repeated attempts for paths that previously failed.
	kerberosErr sync.Map // map[string]error

	// kerberosCredModTime tracks the last known modification time of ccache files.
	// Used to detect changes and trigger credential refresh.
	kerberosCredModTime sync.Map // map[string]time.Time
)

var (
	loadCCacheFunc      = credentials.LoadCCache
	newClientFromCCache = client.NewFromCCache
	loadKrbConfig       = loadKerberosConfig
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

	// Check if the ccache file is modified since last check
	stat, statErr := os.Stat(ccachePath)
	if statErr != nil {
		kerberosErr.Store(ccachePath, statErr)
		return nil, statErr
	}
	mtime := stat.ModTime()

	if oldCredModTimeVal, ok := kerberosCredModTime.Load(ccachePath); ok {
		if oldMtime, ok := oldCredModTimeVal.(time.Time); ok && oldMtime.Equal(mtime) {
			// ccache hasn't changed — return cached client or error
			if errVal, ok := kerberosErr.Load(ccachePath); ok {
				return nil, errVal.(error)
			}
			if clientVal, ok := kerberosClient.Load(ccachePath); ok {
				return clientVal.(*client.Client), nil
			}
		}
	}

	// ccache changed or no valid cached client — reload credentials
	cfg, err := loadKrbConfig()
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
	}
	ccache, err := loadCCacheFunc(ccachePath)
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
	}
	cl, err := newClientFromCCache(ccache, cfg)
	if err != nil {
		kerberosErr.Store(ccachePath, err)
		return nil, err
	}

	kerberosClient.Store(ccachePath, cl)
	kerberosErr.Delete(ccachePath)
	kerberosCredModTime.Store(ccachePath, mtime)
	return cl, nil
}
