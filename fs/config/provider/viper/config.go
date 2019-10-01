package viper

import (
	"bytes"
	"io"
	"log"
	"strings"

	"github.com/rclone/rclone/fs/config"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v2"
)

const (
	RemotesPrefix = "remotes"
)

var (
	emptyStringInterfaceMap = map[string]interface{}{}
)

// Register registers the ConfigProvider
func Register() {
	config.RegisterConfigProvider(&config.ProviderDefinition{
		NewFunc:   NewViperProvider,
		FileTypes: []string{"yaml", "yml"},
	})
}

// NewViperProvider creates an instance of the Viper ConfigProvider
func NewViperProvider() config.Provider {
	return &viperConfig{}
}

type viperConfig struct {
}

func (c *viperConfig) GetString(key string) string {
	return c.GetViper().GetString(key)
}

func (c *viperConfig) SetString(key string, value string) {
	c.GetViper().Set(key, value)
}

func (c *viperConfig) Load(r io.Reader) error {
	viper.SetConfigType("yaml")
	return viper.ReadConfig(r)
}

func (c *viperConfig) Save(w io.Writer) error {
	out, err := yaml.Marshal(viper.AllSettings())
	if err != nil {
		return err
	}
	_, err = w.Write(out)
	return err
}

func (c *viperConfig) GetRemoteConfig() config.RemoteConfig {
	return c
}

func (c *viperConfig) GetConfig() map[string]interface{} {
	return viper.AllSettings()
}

func (c *viperConfig) String() string {
	buf := bytes.Buffer{}
	err := c.Save(&buf)
	if err != nil {
		log.Fatalf("error stringifying config: %v", err)
		return ""
	}
	return buf.String()
}

func (c *viperConfig) GetViper() *viper.Viper {
	return viper.GetViper()
}

func (c *viperConfig) GetRemotes() []string {
	var remotes []string
	remoteEntries := viper.GetStringMap(RemotesPrefix)
	for key, _ := range remoteEntries {
		remotes = append(remotes, key)
	}

	return remotes
}

func (c *viperConfig) HasRemote(remote string) bool {
	return viper.IsSet(getConfigKey(RemotesPrefix, remote))
}

func (c *viperConfig) GetRemote(remote string) config.Section {
	if c.HasRemote(remote) {
		return newSection(viper.GetViper(), getConfigKey(RemotesPrefix, remote))
	}

	return nil
}

func (c *viperConfig) CreateRemote(remote string) config.Section {
	if c.HasRemote(remote) {
		c.DeleteRemote(remote)
	}

	viper.Set(getConfigKey(RemotesPrefix, remote), emptyStringInterfaceMap)

	return c.GetRemote(remote)
}

func (c *viperConfig) DeleteRemote(remote string) {
	viper.Set(getConfigKey(RemotesPrefix, remote), nil)
}

func (c *viperConfig) RenameRemote(oldName string, newName string) {
	c.CopyRemote(oldName, newName)
	c.DeleteRemote(oldName)
}

func (c *viperConfig) CopyRemote(source string, destination string) {
	if !c.HasRemote(source) {
		c.CreateRemote(source)
	}
	if !c.HasRemote(destination) {
		c.CreateRemote(destination)
	}
	sourceConfig := viper.Sub(getConfigKey(RemotesPrefix, source))
	newSection := viper.Sub(getConfigKey(RemotesPrefix, destination))

	for k, v := range sourceConfig.AllSettings() {
		newSection.Set(k, v)
	}
}

func getConfigKey(args ...string) string {
	return strings.Join(args, ".")
}

var (
	_ config.Provider       = (*viperConfig)(nil)
	_ config.RemoteConfig   = (*viperConfig)(nil)
	_ config.GlobalProvider = (*viperConfig)(nil)
)
