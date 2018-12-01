package serve

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"path"

	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/accounting"
	"github.com/ncw/rclone/lib/rest"
	"github.com/gobuffalo/packr/v2"
)

// DirEntry is a directory entry
type DirEntry struct {
	remote string
	URL    string
	Leaf   string
}

// Directory represents a directory
type Directory struct {
	DirRemote string
	Title     string
	Entries   []DirEntry
	Query     string
}

// NewDirectory makes an empty Directory
func NewDirectory(dirRemote string) *Directory {
	d := &Directory{
		DirRemote: dirRemote,
		Title:     fmt.Sprintf("Directory listing of /%s", dirRemote),
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

// Error returns an http.StatusInternalServerError and logs the error
func Error(what interface{}, w http.ResponseWriter, text string, err error) {
	fs.CountError(err)
	fs.Errorf(what, "%s: %v", text, err)
	http.Error(w, text+".", http.StatusInternalServerError)
}

// Serve serves a directory
func (d *Directory) Serve(w http.ResponseWriter, r *http.Request) {
	// Account the transfer
	accounting.Stats.Transferring(d.DirRemote)
	defer accounting.Stats.DoneTransferring(d.DirRemote, true)

	fs.Infof(d.DirRemote, "%s: Serving directory", r.RemoteAddr)
	err := indexTemplate.Execute(w, d)
	if err != nil {
		Error(d.DirRemote, w, "Failed to render template", err)
		return
	}
}

var box = packr.New("templates", "./templates")
var indexPage, err = box.FindString("index.html")

// indexTemplate is the instantiated indexPage
var indexTemplate = template.Must(template.New("index").Parse(indexPage))
