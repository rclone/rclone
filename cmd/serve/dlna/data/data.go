//go:generate go run assets_generate.go
// The "go:generate" directive compiles static assets by running assets_generate.go

package data

import (
	"io/ioutil"
	"text/template"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
)

// GetTemplate returns the rootDesc XML template
func GetTemplate() (tpl *template.Template, err error) {
	templateFile, err := Assets.Open("rootDesc.xml.tmpl")
	if err != nil {
		return nil, errors.Wrap(err, "get template open")
	}

	defer fs.CheckClose(templateFile, &err)

	templateBytes, err := ioutil.ReadAll(templateFile)
	if err != nil {
		return nil, errors.Wrap(err, "get template read")
	}

	var templateString = string(templateBytes)

	tpl, err = template.New("rootDesc").Parse(templateString)
	if err != nil {
		return nil, errors.Wrap(err, "get template parse")
	}

	return
}
