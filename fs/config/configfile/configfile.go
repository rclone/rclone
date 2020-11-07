package configfile

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Unknwon/goconfig"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// GoConfig implements config file saving using a simple ini based
// format.
type GoConfig struct {
	*goconfig.ConfigFile
}

// Load the config from permanent storage
func (gc *GoConfig) Load() error {
	b, err := os.Open(config.ConfigPath)
	if err != nil {
		if os.IsNotExist(err) {
			return config.ErrorConfigFileNotFound
		}
		return err
	}
	defer fs.CheckClose(b, &err)

	cryptReader, err := config.Decrypt(b)
	if err != nil {
		return err
	}

	if gc.ConfigFile == nil {
		c, err := goconfig.LoadFromReader(cryptReader)
		if err != nil {
			return err
		}

		gc.ConfigFile = c
	} else {
		return gc.ReloadData(cryptReader)
	}

	return nil
}

// Save the config to permanent storage
func (gc *GoConfig) Save() error {
	dir, name := filepath.Split(config.ConfigPath)
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return errors.Wrap(err, "failed to create config directory")
	}
	f, err := ioutil.TempFile(dir, name)
	if err != nil {
		return errors.Errorf("Failed to create temp file for new config: %v", err)
	}
	defer func() {
		_ = f.Close()
		if err := os.Remove(f.Name()); err != nil && !os.IsNotExist(err) {
			fs.Errorf(nil, "Failed to remove temp config file: %v", err)
		}
	}()

	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(gc.ConfigFile, &buf); err != nil {
		return errors.Errorf("Failed to save config file: %v", err)
	}

	if err := config.Encrypt(&buf, f); err != nil {
		return err
	}

	_ = f.Sync()
	err = f.Close()
	if err != nil {
		return errors.Errorf("Failed to close config file: %v", err)
	}

	var fileMode os.FileMode = 0600
	info, err := os.Stat(config.ConfigPath)
	if err != nil {
		fs.Debugf(nil, "Using default permissions for config file: %v", fileMode)
	} else if info.Mode() != fileMode {
		fs.Debugf(nil, "Keeping previous permissions for config file: %v", info.Mode())
		fileMode = info.Mode()
	}

	attemptCopyGroup(config.ConfigPath, f.Name())

	err = os.Chmod(f.Name(), fileMode)
	if err != nil {
		fs.Errorf(nil, "Failed to set permissions on config file: %v", err)
	}

	if err = os.Rename(config.ConfigPath, config.ConfigPath+".old"); err != nil && !os.IsNotExist(err) {
		return errors.Errorf("Failed to move previous config to backup location: %v", err)
	}
	if err = os.Rename(f.Name(), config.ConfigPath); err != nil {
		return errors.Errorf("Failed to move newly written config from %s to final location: %v", f.Name(), err)
	}
	if err := os.Remove(config.ConfigPath + ".old"); err != nil && !os.IsNotExist(err) {
		fs.Errorf(nil, "Failed to remove backup config file: %v", err)
	}

	return nil
}

// Serialize the config into a string
func (gc *GoConfig) Serialize() (string, error) {
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(gc.ConfigFile, &buf); err != nil {
		return "", errors.Errorf("Failed to save config file: %v", err)
	}

	return buf.String(), nil
}

// HasSection returns true if section exists in the config file
func (gc *GoConfig) HasSection(section string) bool {
	_, err := gc.ConfigFile.GetSection(section)
	if err != nil {
		return false
	}
	return true
}

// DeleteSection removes the named section and all config from the
// config file
func (gc *GoConfig) DeleteSection(section string) {
	gc.ConfigFile.DeleteSection(section)
}

// SetValue sets the value under key in section
func (gc *GoConfig) SetValue(section string, key string, value string) {
	gc.ConfigFile.SetValue(section, key, value)
}

// Check the interfaces are satisfied
var (
	_ config.File = (*GoConfig)(nil)
)
