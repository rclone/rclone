package goconfig

import (
	"bytes"
	"io"
	"log"

	"github.com/Unknwon/goconfig"
	"github.com/rclone/rclone/fs/config"
)

// Register registers the ConfigProvider
func Register() {
	config.RegisterConfigProvider(&config.ProviderDefinition{
		NewFunc:   NewGoConfigProvider,
		FileTypes: []string{"ini", "conf"},
	})
}

// NewGoConfigProvider creates an instance of the GoConfig ConfigProvider
func NewGoConfigProvider() config.Provider {
	return &goConfig{}
}

type goConfig struct {
	config *goconfig.ConfigFile
}

func (g *goConfig) String() string {
	buf := bytes.Buffer{}
	err := g.Save(&buf)
	if err != nil {
		log.Fatalf("error stringifying config: %v", err)
		return ""
	}
	return buf.String()
}

func (g *goConfig) Load(r io.Reader) error {
	c, err := goconfig.LoadFromReader(r)
	if err != nil {
		return err
	}
	g.config = c
	return nil
}

func (g *goConfig) Save(w io.Writer) error {
	return goconfig.SaveConfigData(g.config, w)
}

func (g *goConfig) GetRemoteConfig() config.RemoteConfig {
	return g
}

var (
	_ config.Provider = (*goConfig)(nil)
)
