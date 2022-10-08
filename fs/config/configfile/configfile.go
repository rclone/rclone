// Package configfile implements a config file loader and saver
package configfile

import (
	"bytes"
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/Unknwon/goconfig"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/lib/file"
)

// Install installs the config file handler
func Install() {
	config.SetData(&Storage{})
}

// Storage implements config.Storage for saving and loading config
// data in a simple INI based file.
type Storage struct {
	mu        sync.Mutex           // to protect the following variables
	gc        *goconfig.ConfigFile // config file loaded - not thread safe
	fiModTime time.Time            // stat of the file when last loaded
	fiSize    int64                // stat of the file size
}

// Check to see if we need to reload the config
//
// mu must be held when calling this
func (s *Storage) _check() {
	if configPath := config.GetConfigPath(); configPath != "" {
		// Check to see if config file has changed since it was last loaded
		fi, err := os.Stat(configPath)
		if err == nil {
			// check to see if config file has changed and if it has, reload it
			if fi.ModTime().After(s.fiModTime) || fi.Size() != s.fiSize {
				fs.Debugf(nil, "Config file has changed externally - reloading")
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
	var sd io.ReadSeekCloser

	// Make sure we have a sensible default even when we error
	defer func() {
		if s.gc == nil {
			s.gc, _ = goconfig.LoadFromReader(bytes.NewReader([]byte{}))
		}
	}()

	configPath := config.GetConfigPath()

	if config.IsConfigCommandIn {
		ctx := context.Background()
		ci := fs.GetConfig(ctx)
		if len(ci.ConfigCommandIn) == 0 {
			return fmt.Errorf("supply arguments to --config-command-in")
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer

		cmd := exec.Command(ci.ConfigCommandIn[0], ci.ConfigCommandIn[1:]...)

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Stdin = os.Stdin

		if err := cmd.Run(); err != nil {
			// One does not always get the stderr returned in the wrapped error.
			fs.Errorf(nil, "Using --config-command-in returned: %v", err)
			if ers := strings.TrimSpace(stderr.String()); ers != "" {
				fs.Errorf(nil, "--config-command-in stderr: %s", ers)
			}
			return fmt.Errorf("command failed: %w", err)
		}
		if outs := stdout.String(); outs != "" {
			fs.Debugf(nil, "--config-command-in stdout: %s", outs)
		}
		cfg := strings.Trim(stdout.String(), "\r\n")
		sd = aws.ReadSeekCloser(strings.NewReader(cfg))

		// Update s.fiModTime and s.fiSize with the current data info
		s.fiModTime, s.fiSize = time.Now(), int64(stdout.Len())
	} else {
		if configPath == "" {
			return config.ErrorConfigFileNotFound
		}

		sd, err = os.Open(configPath)
		if err != nil {
			if os.IsNotExist(err) {
				return config.ErrorConfigFileNotFound
			}
			return err
		}

		// Update s.fiModTime and s.fiSize with the current file info
		fi, _ := os.Stat(configPath)
		s.fiModTime, s.fiSize = fi.ModTime(), fi.Size()
	}
	defer fs.CheckClose(sd, &err)

	cryptReader, err := config.Decrypt(sd)
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

// _save the config from permanent storage, decrypting if necessary
//
// mu must be held when calling this
func (s *Storage) _save() (err error) {
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
		return fmt.Errorf("failed to save config file: %w", err)
	}

	configPath := config.GetConfigPath()

	if config.IsConfigCommandOut {
		ctx := context.Background()
		ci := fs.GetConfig(ctx)
		if len(ci.ConfigCommandOut) == 0 {
			return fmt.Errorf("supply arguments to --config-command-out")
		}

		var stdout bytes.Buffer
		var stderr bytes.Buffer
		var td bytes.Buffer

		if err := config.Encrypt(&buf, &td); err != nil {
			return err
		}
		fiSize := td.Len()

		cmd := exec.Command(ci.ConfigCommandOut[0], ci.ConfigCommandOut[1:]...)

		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		cmd.Stdin = &td

		if err := cmd.Run(); err != nil {
			// One does not always get the stderr returned in the wrapped error.
			fs.Errorf(nil, "Using --config-command-out returned: %v", err)
			if ers := strings.TrimSpace(stderr.String()); ers != "" {
				fs.Errorf(nil, "--config-command-out stderr: %s", ers)
			}
			return fmt.Errorf("config-command-out failed: %w", err)
		}
		if outs := stdout.String(); outs != "" {
			fs.Debugf(nil, "--config-command-out stdout: %s", outs)
		}

		// Update s.fiModTime and s.fiSize with the newly written file
		s.fiModTime, s.fiSize = time.Now(), int64(fiSize)
	} else {
		if configPath == "" {
			return fmt.Errorf("failed to save config file, path is empty")
		}

		dir, name := filepath.Split(configPath)
		err := file.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return fmt.Errorf("failed to create config directory: %w", err)
		}
		td, err := os.CreateTemp(dir, name)
		if err != nil {
			return fmt.Errorf("failed to create temp file for new config: %w", err)
		}
		defer func() {
			_ = td.Close()
			if err := os.Remove(td.Name()); err != nil && !os.IsNotExist(err) {
				fs.Errorf(nil, "failed to remove temp config file: %v", err)
			}
		}()

		if err := config.Encrypt(&buf, td); err != nil {
			return err
		}

		err = td.Sync()
		if err != nil {
			return fmt.Errorf("failed to write config file to disk: %w", err)
		}
		err = td.Close()
		if err != nil {
			return fmt.Errorf("failed to close config file: %w", err)
		}

		var fileMode os.FileMode = 0600
		info, err := os.Stat(configPath)
		if err != nil {
			fs.Debugf(nil, "Using default permissions for config file: %v", fileMode)
		} else if info.Mode() != fileMode {
			fs.Debugf(nil, "Keeping previous permissions for config file: %v", info.Mode())
			fileMode = info.Mode()
		}

		attemptCopyGroup(configPath, td.Name())

		err = os.Chmod(td.Name(), fileMode)
		if err != nil {
			fs.Errorf(nil, "Failed to set permissions on config file: %v", err)
		}

		if err = os.Rename(configPath, configPath+".old"); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("failed to move previous config to backup location: %w", err)
		}
		if err = os.Rename(td.Name(), configPath); err != nil {
			return fmt.Errorf("failed to move newly written config from %s to final location: %v", td.Name(), err)
		}
		if err := os.Remove(configPath + ".old"); err != nil && !os.IsNotExist(err) {
			fs.Errorf(nil, "Failed to remove backup config file: %v", err)
		}

		// Update s.fiModTime and s.fiSize with the newly written file
		fi, _ := os.Stat(configPath)
		s.fiModTime, s.fiSize = fi.ModTime(), fi.Size()
	}

	return nil
}

// Save the config to permanent storage, encrypting if necessary
func (s *Storage) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s._save()
}

// Serialize the config into a string
func (s *Storage) Serialize() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	var buf bytes.Buffer
	if err := goconfig.SaveConfigData(s.gc, &buf); err != nil {
		return "", fmt.Errorf("failed to save config file: %w", err)
	}

	return buf.String(), nil
}

// HasSection returns true if section exists in the config file
func (s *Storage) HasSection(section string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	_, err := s.gc.GetSection(section)
	return err == nil
}

// DeleteSection removes the named section and all config from the
// config file
func (s *Storage) DeleteSection(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	s.gc.DeleteSection(section)
}

// GetSectionList returns a slice of strings with names for all the
// sections
func (s *Storage) GetSectionList() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	return s.gc.GetSectionList()
}

// GetKeyList returns the keys in this section
func (s *Storage) GetKeyList(section string) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	return s.gc.GetKeyList(section)
}

// GetValue returns the key in section with a found flag
func (s *Storage) GetValue(section string, key string) (value string, found bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	value, err := s.gc.GetValue(section, key)
	if err != nil {
		return "", false
	}
	return value, true
}

// SetValue sets the value under key in section
func (s *Storage) SetValue(section string, key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	if strings.HasPrefix(section, ":") {
		fs.Logf(nil, "Can't save config %q for on the fly backend %q", key, section)
		return
	}
	s.gc.SetValue(section, key, value)
}

// DeleteKey removes the key under section
func (s *Storage) DeleteKey(section string, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	s._check()
	return s.gc.DeleteKey(section, key)
}

// Check the interface is satisfied
var _ config.Storage = (*Storage)(nil)
