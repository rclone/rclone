package viper

import (
	"strings"

	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/viper"
)

type section struct {
	v        *viper.Viper
	basePath string
}

func (s *section) GetKeys() []string {
	var keys []string
	for _, k := range s.v.AllKeys() {
		if strings.HasPrefix(k, s.basePath) {
			keys = append(keys, k)
		}
	}

	return keys
}

func (s *section) GetConfig() map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range s.v.Sub(s.basePath).AllSettings() {
		data[k] = v
	}

	return data
}

func (s *section) Remove(name string) {
	s.v.Set(getConfigKey(s.basePath, name), nil)
}

func (s *section) Get(name string) interface{} {
	return s.v.Get(getConfigKey(s.basePath, name))
}

func (s *section) GetString(name string) string {
	return s.v.GetString(getConfigKey(s.basePath, name))
}

func (s *section) GetStringDefault(name string, default_ string) string {
	if s.v.IsSet(getConfigKey(s.basePath, name)) {
		return s.v.GetString(getConfigKey(s.basePath, name))
	} else {
		return default_
	}
}

func (s *section) GetInt(name string) int {
	return s.v.GetInt(getConfigKey(s.basePath, name))
}

func (s *section) GetStringSlice(name string) []string {
	return s.v.GetStringSlice(getConfigKey(s.basePath, name))
}

func (s *section) GetStringMap(name string) map[string]string {
	return s.v.GetStringMapString(getConfigKey(s.basePath, name))
}

func (s *section) SetString(name string, value string) {
	s.Set(name, value)
}

func (s *section) SetInt(name string, value int) {
	s.Set(name, value)
}

func (s *section) SetStringSlice(name string, value []string) {
	s.Set(name, value)
}

func (s *section) SetStringMap(name string, value map[string]string) {
	s.Set(name, value)
}

func (s *section) Set(name string, value interface{}) {
	s.v.Set(getConfigKey(s.basePath, name), value)
}

func newSection(v *viper.Viper, basePath string) *section {
	return &section{
		v:        v,
		basePath: basePath,
	}
}

var (
	_ config.Section = (*section)(nil)
)
