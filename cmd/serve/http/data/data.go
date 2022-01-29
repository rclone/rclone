//go:generate go run assets_generate.go
// The "go:generate" directive compiles static assets by running assets_generate.go

package data

import (
	"fmt"
	"html/template"
	"io/ioutil"
	"time"

	"github.com/spf13/pflag"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
)

// Help describes the options for the serve package
var Help = `
#### Template

--template allows a user to specify a custom markup template for http
and webdav serve functions.  The server exports the following markup
to be used within the template to server pages:

| Parameter   | Description |
| :---------- | :---------- |
| .Name       | The full path of a file/directory. |
| .Title      | Directory listing of .Name |
| .Sort       | The current sort used.  This is changeable via ?sort= parameter |
|             | Sort Options: namedirfirst,name,size,time (default namedirfirst) |
| .Order      | The current ordering used.  This is changeable via ?order= parameter |
|             | Order Options: asc,desc (default asc) |
| .Query      | Currently unused. |
| .Breadcrumb | Allows for creating a relative navigation |
|-- .Link     | The relative to the root link of the Text. |
|-- .Text     | The Name of the directory. |
| .Entries    | Information about a specific file/directory. |
|-- .URL      | The 'url' of an entry.  |
|-- .Leaf     | Currently same as 'URL' but intended to be 'just' the name. |
|-- .IsDir    | Boolean for if an entry is a directory or not. |
|-- .Size     | Size in Bytes of the entry. |
|-- .ModTime  | The UTC timestamp of an entry. |
`

// Options for the templating functionality
type Options struct {
	Template string
}

// AddFlags for the templating functionality
func AddFlags(flagSet *pflag.FlagSet, prefix string, Opt *Options) {
	flags.StringVarP(flagSet, &Opt.Template, prefix+"template", "", Opt.Template, "User-specified template")
}

// AfterEpoch returns the time since the epoch for the given time
func AfterEpoch(t time.Time) bool {
	return t.After(time.Time{})
}

// GetTemplate returns the HTML template for serving directories via HTTP/Webdav
func GetTemplate(tmpl string) (tpl *template.Template, err error) {
	var templateString string
	if tmpl == "" {
		templateFile, err := Assets.Open("index.html")
		if err != nil {
			return nil, fmt.Errorf("get template open: %w", err)
		}

		defer fs.CheckClose(templateFile, &err)

		templateBytes, err := ioutil.ReadAll(templateFile)
		if err != nil {
			return nil, fmt.Errorf("get template read: %w", err)
		}

		templateString = string(templateBytes)

	} else {
		templateFile, err := ioutil.ReadFile(tmpl)
		if err != nil {
			return nil, fmt.Errorf("get template open: %w", err)
		}

		templateString = string(templateFile)
	}

	funcMap := template.FuncMap{
		"afterEpoch": AfterEpoch,
	}
	tpl, err = template.New("index").Funcs(funcMap).Parse(templateString)
	if err != nil {
		return nil, fmt.Errorf("get template parse: %w", err)
	}

	return
}
