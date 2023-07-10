package http

import (
	"bytes"
	"embed"
	"html/template"
	"log"
	"os"
	"time"

	"github.com/spf13/pflag"

	"github.com/rclone/rclone/fs/config/flags"
)

// TemplateHelp returns a string that describes how to use a custom template
func TemplateHelp(prefix string) string {
	help := `
#### Template

` + "`--{{ .Prefix }}template`" + ` allows a user to specify a custom markup template for HTTP
and WebDAV serve functions.  The server exports the following markup
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

	tmpl, err := template.New("template help").Parse(help)
	if err != nil {
		log.Fatal("Fatal error parsing template", err)
	}

	data := struct {
		Prefix string
	}{
		Prefix: prefix,
	}
	buf := &bytes.Buffer{}
	err = tmpl.Execute(buf, data)
	if err != nil {
		log.Fatal("Fatal error executing template", err)
	}
	return buf.String()
}

// TemplateConfig for the templating functionality
type TemplateConfig struct {
	Path string
}

// AddFlagsPrefix for the templating functionality
func (cfg *TemplateConfig) AddFlagsPrefix(flagSet *pflag.FlagSet, prefix string) {
	flags.StringVarP(flagSet, &cfg.Path, prefix+"template", "", cfg.Path, "User-specified template", prefix)
}

// AddTemplateFlagsPrefix for the templating functionality
func AddTemplateFlagsPrefix(flagSet *pflag.FlagSet, prefix string, cfg *TemplateConfig) {
	cfg.AddFlagsPrefix(flagSet, prefix)
}

// DefaultTemplateCfg returns a new config which can be customized by command line flags
func DefaultTemplateCfg() TemplateConfig {
	return TemplateConfig{}
}

// AfterEpoch returns the time since the epoch for the given time
func AfterEpoch(t time.Time) bool {
	return t.After(time.Time{})
}

// Assets holds the embedded filesystem for the default template
//
//go:embed templates
var Assets embed.FS

// GetTemplate returns the HTML template for serving directories via HTTP/WebDAV
func GetTemplate(tmpl string) (*template.Template, error) {
	var readFile = os.ReadFile
	if tmpl == "" {
		tmpl = "templates/index.html"
		readFile = Assets.ReadFile
	}

	data, err := readFile(tmpl)
	if err != nil {
		return nil, err
	}

	funcMap := template.FuncMap{
		"afterEpoch": AfterEpoch,
	}

	tpl, err := template.New("index").Funcs(funcMap).Parse(string(data))
	if err != nil {
		return nil, err
	}

	return tpl, nil
}
