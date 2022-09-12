// NewFs and its helpers

package fs

import (
	"context"
	"crypto/md5"
	"encoding/base64"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/fspath"
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
		suffix := base64.RawURLEncoding.EncodeToString(md5sumBinary[:])
		// 5 characters length is 5*6 = 30 bits of base64
		const maxLength = 5
		if len(suffix) > maxLength {
			suffix = suffix[:maxLength]
		}
		suffix = "{" + suffix + "}"
		Debugf(configName, "detected overridden config - adding %q suffix to name", suffix)
		// Add the suffix to the config name
		//
		// These need to work as filesystem names as the VFS cache will use them
		configName += suffix
	}
	f, err := fsInfo.NewFs(ctx, configName, fsPath, config)
	if f != nil && (err == nil || err == ErrorIsFile) {
		addReverse(f, fsInfo)
	}
	return f, err
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
	config = ConfigMap(fsInfo, configName, connectionStringConfig)
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
			m := ConfigMap(nil, configName, parsed.Config)
			fsName, ok = m.Get("type")
			if !ok {
				return nil, "", "", nil, ErrorNotFoundInConfigFile
			}
		}
	} else {
		fsName = "local"
		configName = "local"
	}
	fsInfo, err = Find(fsName)
	return fsInfo, configName, fsPath, parsed.Config, err
}

// ConfigString returns a canonical version of the config string used
// to configure the Fs as passed to fs.NewFs
func ConfigString(f Fs) string {
	name := f.Name()
	root := f.Root()
	if name == "local" && f.Features().IsLocal {
		return root
	}
	return name + ":" + root
}

// TemporaryLocalFs creates a local FS in the OS's temporary directory.
//
// No cleanup is performed, the caller must call Purge on the Fs themselves.
func TemporaryLocalFs(ctx context.Context) (Fs, error) {
	path, err := ioutil.TempDir("", "rclone-spool")
	if err == nil {
		err = os.Remove(path)
	}
	if err != nil {
		return nil, err
	}
	path = filepath.ToSlash(path)
	return NewFs(ctx, path)
}
