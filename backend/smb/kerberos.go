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

// KerberosFactory encapsulates dependencies and caches for Kerberos clients.
type KerberosFactory struct {
	// clientCache caches Kerberos clients keyed by resolved ccache path.
	// Clients are reused unless the associated ccache file changes.
	clientCache sync.Map // map[string]*client.Client

	// errCache caches errors encountered when loading Kerberos clients.
	// Prevents repeated attempts for paths that previously failed.
	errCache sync.Map // map[string]error

	// modTimeCache tracks the last known modification time of ccache files.
	// Used to detect changes and trigger credential refresh.
	modTimeCache sync.Map // map[string]time.Time

	loadCCache func(string) (*credentials.CCache, error)
	newClient  func(*credentials.CCache, *config.Config, ...func(*client.Settings)) (*client.Client, error)
	loadConfig func() (*config.Config, error)
}

// NewKerberosFactory creates a new instance of KerberosFactory with default dependencies.
func NewKerberosFactory() *KerberosFactory {
	return &KerberosFactory{
		loadCCache: credentials.LoadCCache,
		newClient:  client.NewFromCCache,
		loadConfig: defaultLoadKerberosConfig,
	}
}

// GetClient returns a cached Kerberos client or creates a new one if needed.
func (kf *KerberosFactory) GetClient(ccachePath string) (*client.Client, error) {
	resolvedPath, err := resolveCcachePath(ccachePath)
	if err != nil {
		return nil, err
	}

	stat, err := os.Stat(resolvedPath)
	if err != nil {
		kf.errCache.Store(resolvedPath, err)
		return nil, err
	}
	mtime := stat.ModTime()

	if oldMod, ok := kf.modTimeCache.Load(resolvedPath); ok {
		if oldTime, ok := oldMod.(time.Time); ok && oldTime.Equal(mtime) {
			if errVal, ok := kf.errCache.Load(resolvedPath); ok {
				return nil, errVal.(error)
			}
			if clientVal, ok := kf.clientCache.Load(resolvedPath); ok {
				return clientVal.(*client.Client), nil
			}
		}
	}

	// Load Kerberos config
	cfg, err := kf.loadConfig()
	if err != nil {
		kf.errCache.Store(resolvedPath, err)
		return nil, err
	}

	// Load ccache
	ccache, err := kf.loadCCache(resolvedPath)
	if err != nil {
		kf.errCache.Store(resolvedPath, err)
		return nil, err
	}

	// Create new client
	cl, err := kf.newClient(ccache, cfg)
	if err != nil {
		kf.errCache.Store(resolvedPath, err)
		return nil, err
	}

	// Cache and return
	kf.clientCache.Store(resolvedPath, cl)
	kf.errCache.Delete(resolvedPath)
	kf.modTimeCache.Store(resolvedPath, mtime)
	return cl, nil
}

// resolveCcachePath resolves the KRB5 ccache path.
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

// defaultLoadKerberosConfig loads Kerberos config from default or env path.
func defaultLoadKerberosConfig() (*config.Config, error) {
	cfgPath := os.Getenv("KRB5_CONFIG")
	if cfgPath == "" {
		cfgPath = "/etc/krb5.conf"
	}
	return config.Load(cfgPath)
}
