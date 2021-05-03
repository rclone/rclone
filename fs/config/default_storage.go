package config

import (
	"encoding/json"
	"sync"
)

// defaultStorage implements config.Storage, providing in-memory config.
// Indexed by section, then key.
type defaultStorage struct {
	mu       sync.RWMutex
	sections map[string]map[string]string
}

func newDefaultStorage() *defaultStorage {
	return &defaultStorage{
		sections: map[string]map[string]string{},
	}
}

// GetSectionList returns a slice of strings with names for all the sections.
func (s *defaultStorage) GetSectionList() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	sections := make([]string, 0, len(s.sections))
	for section := range s.sections {
		sections = append(sections, section)
	}
	return sections
}

// HasSection returns true if section exists in the config.
func (s *defaultStorage) HasSection(section string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, hasSection := s.sections[section]
	return hasSection
}

// DeleteSection deletes the specified section.
func (s *defaultStorage) DeleteSection(section string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sections, section)
}

// GetKeyList returns the keys in this section.
func (s *defaultStorage) GetKeyList(section string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	theSection := s.sections[section]
	keys := make([]string, 0, len(theSection))
	for key := range theSection {
		keys = append(keys, key)
	}
	return keys
}

// GetValue returns the key in section with a found flag.
func (s *defaultStorage) GetValue(section string, key string) (value string, found bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	theSection, hasSection := s.sections[section]
	if !hasSection {
		return "", false
	}
	value, hasValue := theSection[key]
	return value, hasValue
}

func (s *defaultStorage) SetValue(section string, key string, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	theSection, hasSection := s.sections[section]
	if !hasSection {
		theSection = map[string]string{}
		s.sections[section] = theSection
	}
	theSection[key] = value
}

func (s *defaultStorage) DeleteKey(section string, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	theSection, hasSection := s.sections[section]
	if !hasSection {
		return false
	}
	_, hasKey := theSection[key]
	if !hasKey {
		return false
	}
	delete(s.sections[section], key)
	return true
}

func (s *defaultStorage) Load() error {
	return nil
}

func (s *defaultStorage) Save() error {
	return nil
}

// Serialize the config into a string
func (s *defaultStorage) Serialize() (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	j, err := json.Marshal(s.sections)
	return string(j), err
}

// Check the interface is satisfied
var _ Storage = newDefaultStorage()
