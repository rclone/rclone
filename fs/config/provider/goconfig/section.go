package goconfig

import (
	"fmt"
	"log"
	"strconv"

	"github.com/Unknwon/goconfig"
	"github.com/rclone/rclone/fs/config"
)

type section struct {
	config *goconfig.ConfigFile
	remote string
}

func (s *section) GetKeys() []string {
	var keys []string
	for k := range s.getRemoteData() {
		keys = append(keys, k)
	}
	return keys
}

func (s *section) GetConfig() map[string]interface{} {
	data := make(map[string]interface{})
	for k, v := range s.getRemoteData() {
		data[k] = v
	}

	return data
}

func (s *section) Remove(name string) {
	s.config.DeleteKey(s.remote, name)
}

func (s *section) Get(name string) interface{} {
	return s.GetStringDefault(name, "")
}

func (s *section) GetString(name string) string {
	return s.GetStringDefault(name, "")
}

func (s *section) GetStringDefault(name string, default_ string) string {
	if v, ok := s.getRemoteData()[name]; ok {
		return v
	}
	return default_
}

func (s *section) SetString(name string, value string) {
	s.config.SetValue(s.remote, name, value)
}

func (s *section) SetInt(name string, value int) {
	s.SetString(name, strconv.Itoa(value))
}

func (s *section) Set(name string, value interface{}) {
	s.SetString(name, fmt.Sprintf("%v", value))
}

func (s *section) getRemoteData() map[string]string {
	data, err := s.config.GetSection(s.remote)
	if err != nil {
		log.Fatalf("couldnt load section: %s", err)
	}
	return data
}

func newSection(config *goconfig.ConfigFile, remote string) config.Section {
	return &section{
		config: config,
		remote: remote,
	}
}

var (
	_ config.Section = (*section)(nil)
)
