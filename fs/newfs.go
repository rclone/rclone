// NewFs and its helpers

package fs

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fspath"
)

// Store the hashes of the overridden config
var (
	overriddenConfigMu sync.Mutex
	overriddenConfig   = make(map[string]string)
)

// NewFs makes a new Fs object from the path
//
// The path is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
//
// On Windows avoid single character remote names as they can be mixed
// up with drive letters.
func NewFs(ctx context.Context, path string) (Fs, error) {
	Debugf(nil, "Creating backend with remote %q", path)
	if ConfigFileHasSection(path) {
		Logf(nil, "%q refers to a local folder, use %q to refer to your remote or %q to hide this warning", path, path+":", "./"+path)
	}
	fsInfo, configName, fsPath, config, err := ConfigFs(path)
	if err != nil {
		return nil, err
	}
	overridden := fsInfo.Options.Overridden(config)
	if len(overridden) > 0 {
		extraConfig := overridden.String()
		//Debugf(nil, "detected overridden config %q", extraConfig)
		md5sumBinary := md5.Sum([]byte(extraConfig))
		configHash := base64.RawURLEncoding.EncodeToString(md5sumBinary[:])
		// 5 characters length is 5*6 = 30 bits of base64
		overriddenConfigMu.Lock()
		var suffix string
		for maxLength := 5; ; maxLength++ {
			suffix = "{" + configHash[:maxLength] + "}"
			existingExtraConfig, ok := overriddenConfig[suffix]
			if !ok || existingExtraConfig == extraConfig {
				break
			}
		}
		Debugf(configName, "detected overridden config - adding %q suffix to name", suffix)
		// Add the suffix to the config name
		//
		// These need to work as filesystem names as the VFS cache will use them
		configName += suffix
		// Store the config suffixes for reversing in ConfigString
		overriddenConfig[suffix] = extraConfig
		overriddenConfigMu.Unlock()
	}
	ctx, err = addConfigToContext(ctx, configName, config)
	if err != nil {
		return nil, err
	}
	f, err := fsInfo.NewFs(ctx, configName, fsPath, config)
	if f != nil && (err == nil || err == ErrorIsFile) {
		addReverse(f, fsInfo)
	}
	return f, err
}

// Add "global" config or "override" to ctx and the global config if required.
//
// This looks through keys prefixed with "global." or "override." in
// config and sets ctx and optionally the global context if "global.".
func addConfigToContext(ctx context.Context, configName string, config configmap.Getter) (newCtx context.Context, err error) {
	overrideConfig := make(configmap.Simple)
	globalConfig := make(configmap.Simple)
	for i := range ConfigOptionsInfo {
		opt := &ConfigOptionsInfo[i]
		globalName := "global." + opt.Name
		value, isSet := config.Get(globalName)
		if isSet {
			// Set both override and global if global
			overrideConfig[opt.Name] = value
			globalConfig[opt.Name] = value
		}
		overrideName := "override." + opt.Name
		value, isSet = config.Get(overrideName)
		if isSet {
			overrideConfig[opt.Name] = value
		}
	}
	if len(overrideConfig) == 0 && len(globalConfig) == 0 {
		return ctx, nil
	}
	newCtx, ci := AddConfig(ctx)
	overrideKeys := slices.Collect(maps.Keys(overrideConfig))
	slices.Sort(overrideKeys)
	globalKeys := slices.Collect(maps.Keys(globalConfig))
	slices.Sort(globalKeys)
	// Set the config in the newCtx
	err = configstruct.Set(overrideConfig, ci)
	if err != nil {
		return ctx, fmt.Errorf("failed to set override config variables %q: %w", overrideKeys, err)
	}
	Debugf(configName, "Set overridden config %q for backend startup", overrideKeys)
	// Set the global context only
	if len(globalConfig) != 0 {
		globalCI := GetConfig(context.Background())
		err = configstruct.Set(globalConfig, globalCI)
		if err != nil {
			return ctx, fmt.Errorf("failed to set global config variables %q: %w", globalKeys, err)
		}
		Debugf(configName, "Set global config %q at backend startup", overrideKeys)
	}
	return newCtx, nil
}

// ConfigFs makes the config for calling NewFs with.
//
// It parses the path which is of the form remote:path
//
// Remotes are looked up in the config file.  If the remote isn't
// found then NotFoundInConfigFile will be returned.
func ConfigFs(path string) (fsInfo *RegInfo, configName, fsPath string, config *configmap.Map, err error) {
	// Parse the remote path
	fsInfo, configName, fsPath, connectionStringConfig, err := ParseRemote(path)
	if err != nil {
		return
	}
	config = ConfigMap(fsInfo.Prefix, fsInfo.Options, configName, connectionStringConfig)
	return
}

// ParseRemote deconstructs a path into configName, fsPath, looking up
// the fsName in the config file (returning NotFoundInConfigFile if not found)
func ParseRemote(path string) (fsInfo *RegInfo, configName, fsPath string, connectionStringConfig configmap.Simple, err error) {
	parsed, err := fspath.Parse(path)
	if err != nil {
		return nil, "", "", nil, err
	}
	configName, fsPath = parsed.Name, parsed.Path
	var fsName string
	var ok bool
	if configName != "" {
		if strings.HasPrefix(configName, ":") {
			fsName = configName[1:]
		} else {
			m := ConfigMap("", nil, configName, parsed.Config)
			fsName, ok = m.Get("type")
			if !ok {
				return nil, "", "", nil, fmt.Errorf("%w (%q)", ErrorNotFoundInConfigFile, configName)
			}
		}
	} else {
		fsName = "local"
		configName = "local"
	}
	fsInfo, err = Find(fsName)
	return fsInfo, configName, fsPath, parsed.Config, err
}

// configString returns a canonical version of the config string used
// to configure the Fs as passed to fs.NewFs
func configString(f Info, full bool) string {
	name := f.Name()
	if open := strings.IndexRune(name, '{'); full && open >= 0 && strings.HasSuffix(name, "}") {
		suffix := name[open:]
		overriddenConfigMu.Lock()
		config, ok := overriddenConfig[suffix]
		overriddenConfigMu.Unlock()
		if ok {
			name = name[:open] + "," + config
		} else {
			Errorf(f, "Failed to find config for suffix %q", suffix)
		}
	}
	root := f.Root()
	if name == "local" && f.Features().IsLocal {
		return root
	}
	return name + ":" + root
}

// ConfigString returns a canonical version of the config string used
// to configure the Fs as passed to fs.NewFs. For Fs with extra
// parameters this will include a canonical {hexstring} suffix.
func ConfigString(f Info) string {
	return configString(f, false)
}

// FullPath returns the full path with remote:path/to/object
// for an object.
func FullPath(o Object) string {
	return fspath.JoinRootPath(ConfigString(o.Fs()), o.Remote())
}

// ConfigStringFull returns a canonical version of the config string
// used to configure the Fs as passed to fs.NewFs. This string can be
// used to re-instantiate the Fs exactly so includes all the extra
// parameters passed in.
func ConfigStringFull(f Fs) string {
	return configString(f, true)
}

// TemporaryLocalFs creates a local FS in the OS's temporary directory.
//
// No cleanup is performed, the caller must call Purge on the Fs themselves.
func TemporaryLocalFs(ctx context.Context) (Fs, error) {
	path, err := os.MkdirTemp("", "rclone-spool")
	if err == nil {
		err = os.Remove(path)
	}
	if err != nil {
		return nil, err
	}
	path = filepath.ToSlash(path)
	return NewFs(ctx, path)
}
