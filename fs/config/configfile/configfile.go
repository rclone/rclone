// Package configfile implements a config file loader and saver
package configfile

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"

	"github.com/Unknwon/goconfig"
	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
)

// Install installs the config file handler
func Install() {
	config.SetData(&Storage{})
}

// Storage implements config.Storage for saving and loading config
// data in a simple INI based file.
type Storage struct {
	gc *goconfig.ConfigFile // config file loaded - thread safe
	mu sync.Mutex           // to protect the following variables
	fi os.FileInfo          // stat of the file when last loaded
}

// Check to see if we need to reload the config
func (s *Storage) check() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if configPath := config.GetConfigPath(); configPath != "" {
		// Check to see if config file has changed since it was last loaded
		fi, err := os.Stat(configPath)
		if err == nil {
			// check to see if config file has changed and if it has, reload it
			if s.fi == nil || !fi.ModTime().Equal(s.fi.ModTime()) || fi.Size() != s.fi.Size() {
				fs.Debugf(nil, "Config file has changed externaly - reloading")
				err := s._load()
				if err != nil {
					fs.Errorf(nil, "Failed to read config file - using previous config: %v", err)
				}
			}
		}
	}
}

// _load the config from permanent storage, decrypting if necessary
//
// mu must be held when calling this
func (s *Storage) _load() (err error) {
	// Make sure we have a sensible default even when we error
	defer func() {
		if s.gc == nil {
			s.gc, _ = goconfig.LoadFromReader(bytes.NewReader([]byte{}))
		}
	}()

	configPath := config.GetConfigPath()
	if configPath == "" {
		return config.ErrorConfigFileNotFound
	}

	fd, err := os.Open(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return config.ErrorConfigFileNotFound
		}
		return err
	}
	defer fs.CheckClose(fd, &err)

	// Update s.fi with the current file info
	s.fi, _ = os.Stat(configPath)

	cryptReader, err := config.Decrypt(fd)
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

// Load the config from permanent storage, decrypting if necessary
func (s *Storage) Load() (err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._load()
}

// Save the config to permanent storage, encrypting if necessary
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	configPath := config.GetConfigPath()
	if configPath == "" {
		return errors.Errorf("Failed to save config file: Path is empty")
	}

	dir, name := filepath.Split(configPath)
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
	info, err := os.Stat(configPath)
	if err != nil {
		fs.Debugf(nil, "Using default permissions for config file: %v", fileMode)
	} else if info.Mode() != fileMode {
		fs.Debugf(nil, "Keeping previous permissions for config file: %v", info.Mode())
		fileMode = info.Mode()
	}

	attemptCopyGroup(configPath, f.Name())

	err = os.Chmod(f.Name(), fileMode)
	if err != nil {
		fs.Errorf(nil, "Failed to set permissions on config file: %v", err)
	}

	if err = os.Rename(configPath, configPath+".old"); err != nil && !os.IsNotExist(err) {
		return errors.Errorf("Failed to move previous config to backup location: %v", err)
	}
	if err = os.Rename(f.Name(), configPath); err != nil {
		return errors.Errorf("Failed to move newly written config from %s to final location: %v", f.Name(), err)
	}
	if err := os.Remove(configPath + ".old"); err != nil && !os.IsNotExist(err) {
		fs.Errorf(nil, "Failed to remove backup config file: %v", err)
	}

	// Update s.fi with the newly written file
	s.fi, _ = os.Stat(configPath)

	return nil
}

// Serialize the config into a string
func (s *Storage) Serialize() (string, error) {
	s.check()
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
		return "", errors.Errorf("Failed to save config file: %v", err)
	}

	return buf.String(), nil
}

// HasSection returns true if section exists in the config file
func (s *Storage) HasSection(section string) bool {
	s.check()
	_, err := s.gc.GetSection(section)
	if err != nil {
		return false
	}
	return true
}

// DeleteSection removes the named section and all config from the
// config file
func (s *Storage) DeleteSection(section string) {
	s.check()
	s.gc.DeleteSection(section)
}

// GetSectionList returns a slice of strings with names for all the
// sections
func (s *Storage) GetSectionList() []string {
	s.check()
	return s.gc.GetSectionList()
}

// GetKeyList returns the keys in this section
func (s *Storage) GetKeyList(section string) []string {
	s.check()
	return s.gc.GetKeyList(section)
}

// GetValue returns the key in section with a found flag
func (s *Storage) GetValue(section string, key string) (value string, found bool) {
	s.check()
	value, err := s.gc.GetValue(section, key)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetValue sets the value under key in section
func (s *Storage) SetValue(section string, key string, value string) {
	s.check()
	s.gc.SetValue(section, key, value)
}

// DeleteKey removes the key under section
func (s *Storage) DeleteKey(section string, key string) bool {
	s.check()
	return s.gc.DeleteKey(section, key)
}

// Check the interface is satisfied
var _ config.Storage = (*Storage)(nil)
