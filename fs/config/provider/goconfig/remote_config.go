package goconfig

import (
	"log"

	"github.com/rclone/rclone/fs/config"
)

func (g *goConfig) GetRemotes() []string {
	return g.config.GetSectionList()
}

func (g *goConfig) HasRemote(remote string) bool {
	for _, v := range g.GetRemotes() {
		if v == remote {
			return true
		}
	}
	return false
}

func (g *goConfig) GetRemote(remote string) config.Section {
	return newSection(g.config, remote)
}

func (g *goConfig) CreateRemote(remote string) config.Section {
	g.config.SetValue(remote, "", "")
	return g.GetRemote(remote)
}

func (g *goConfig) DeleteRemote(name string) {
	g.config.DeleteSection(name)
}

func (g *goConfig) RenameRemote(oldName string, newName string) {
	g.CopyRemote(oldName, newName)
	g.config.DeleteSection(oldName)
}

func (g *goConfig) CopyRemote(source string, destination string) {
	data, err := g.config.GetSection(source)
	if err != nil {
		log.Fatalf("couldnt load section: %s", err)
	}

	for k, v := range data {
		g.config.SetValue(destination, k, v)
	}
}

var (
	_ config.RemoteConfig = (*goConfig)(nil)
)
