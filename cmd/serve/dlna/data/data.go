// Package data provides utilities for DLNA server.
// The "go:generate" directive compiles static assets by running assets_generate.go
//
//go:generate go run assets_generate.go
package data

import (
	"fmt"
	"io/ioutil"
	"text/template"

	"github.com/rclone/rclone/fs"
)

// GetTemplate returns the rootDesc XML template
func GetTemplate() (tpl *template.Template, err error) {
	templateFile, err := Assets.Open("rootDesc.xml.tmpl")
	if err != nil {
		return nil, fmt.Errorf("get template open: %w", err)
	}

	defer fs.CheckClose(templateFile, &err)

	templateBytes, err := ioutil.ReadAll(templateFile)
	if err != nil {
		return nil, fmt.Errorf("get template read: %w", err)
	}

	var templateString = string(templateBytes)

	tpl, err = template.New("rootDesc").Parse(templateString)
	if err != nil {
		return nil, fmt.Errorf("get template parse: %w", err)
	}

	return
}
