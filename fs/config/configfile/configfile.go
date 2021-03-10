package configfile

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/Unknwon/goconfig"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// LoadConfig installs the config file handler and calls config.LoadConfig
func LoadConfig(ctx context.Context) {
	config.Data = &Storage{}
	config.LoadConfig(ctx)
}

// Storage implements config.Storage for saving and loading config
// data in a simple INI based file.
type Storage struct {
	gc *goconfig.ConfigFile
}

// Load the config from permanent storage, decrypting if necessary
func (s *Storage) Load() (err error) {
	// Make sure we have a sensible default even when we error
	defer func() {
		if err != nil {
			s.gc, _ = goconfig.LoadFromReader(bytes.NewReader([]byte{}))
		}
	}()

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

	gc, err := goconfig.LoadFromReader(cryptReader)
	if err != nil {
		return err
	}
	s.gc = gc

	return nil
}

// Save the config to permanent storage, encrypting if necessary
func (s *Storage) Save() error {
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
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
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
func (s *Storage) Serialize() (string, error) {
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
		return "", errors.Errorf("Failed to save config file: %v", err)
	}

	return buf.String(), nil
}

// HasSection returns true if section exists in the config file
func (s *Storage) HasSection(section string) bool {
	_, err := s.gc.GetSection(section)
	if err != nil {
		return false
	}
	return true
}

// DeleteSection removes the named section and all config from the
// config file
func (s *Storage) DeleteSection(section string) {
	s.gc.DeleteSection(section)
}

// GetSectionList returns a slice of strings with names for all the
// sections
func (s *Storage) GetSectionList() []string {
	return s.gc.GetSectionList()
}

// GetKeyList returns the keys in this section
func (s *Storage) GetKeyList(section string) []string {
	return s.gc.GetKeyList(section)
}

// GetValue returns the key in section with a found flag
func (s *Storage) GetValue(section string, key string) (value string, found bool) {
	value, err := s.gc.GetValue(section, key)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetValue sets the value under key in section
func (s *Storage) SetValue(section string, key string, value string) {
	s.gc.SetValue(section, key, value)
}

// DeleteKey removes the key under section
func (s *Storage) DeleteKey(section string, key string) bool {
	return s.gc.DeleteKey(section, key)
}

// Check the interface is satisfied
var _ config.Storage = (*Storage)(nil)
