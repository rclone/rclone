package serve

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/rest"
)

// DirEntry is a directory entry
type DirEntry struct {
	remote string
	URL    string
	Leaf   string
}

// Directory represents a directory
type Directory struct {
	DirRemote    string
	Title        string
	Entries      []DirEntry
	Query        string
	HTMLTemplate *template.Template
}

// NewDirectory makes an empty Directory
func NewDirectory(dirRemote string, htmlTemplate *template.Template) *Directory {
	d := &Directory{
		DirRemote:    dirRemote,
		Title:        fmt.Sprintf("Directory listing of /%s", dirRemote),
		HTMLTemplate: htmlTemplate,
	}
	return d
}

// SetQuery sets the query parameters for each URL
func (d *Directory) SetQuery(queryParams url.Values) *Directory {
	d.Query = ""
	if len(queryParams) > 0 {
		d.Query = "?" + queryParams.Encode()
	}
	return d
}

// AddEntry adds an entry to that directory
func (d *Directory) AddEntry(remote string, isDir bool) {
	leaf := path.Base(remote)
	if leaf == "." {
		leaf = ""
	}
	urlRemote := leaf
	if isDir {
		leaf += "/"
		urlRemote += "/"
	}
	d.Entries = append(d.Entries, DirEntry{
		remote: remote,
		URL:    rest.URLPathEscape(urlRemote) + d.Query,
		Leaf:   leaf,
	})
}

// Error logs the error and if a ResponseWriter is given it writes a http.StatusInternalServerError
func Error(what interface{}, w http.ResponseWriter, text string, err error) {
	err = fs.CountError(err)
	fs.Errorf(what, "%s: %v", text, err)
	if w != nil {
		http.Error(w, text+".", http.StatusInternalServerError)
	}
}

// Serve serves a directory
func (d *Directory) Serve(w http.ResponseWriter, r *http.Request) {
	// Account the transfer
	tr := accounting.Stats(r.Context()).NewTransferRemoteSize(d.DirRemote, -1)
	defer tr.Done(nil)

	fs.Infof(d.DirRemote, "%s: Serving directory", r.RemoteAddr)

	buf := &bytes.Buffer{}
	err := d.HTMLTemplate.Execute(buf, d)
	if err != nil {
		Error(d.DirRemote, w, "Failed to render template", err)
		return
	}
	_, err = buf.WriteTo(w)
	if err != nil {
		Error(d.DirRemote, nil, "Failed to drain template buffer", err)
	}
}
