package serve

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/rest"
)

// DirEntry is a directory entry
type DirEntry struct {
	remote   string
	URL      string
	ZipURL   string
	Leaf     string
	IsDir    bool
	Size     int64
	ModTime  time.Time
	MimeType string
}

// Directory represents a directory
type Directory struct {
	DirRemote    string
	Title        string
	Name         string
	ZipURL       string
	DisableZip   bool
	Entries      []DirEntry
	Query        string
	HTMLTemplate *template.Template
	Breadcrumb   []Crumb
	Sort         string
	Order        string
	Static       bool   // rendering for a static site, so no "up" link at the root
	LinkIndex    string // if set, appended to directory links, e.g. "index.html"
}

// Crumb is a breadcrumb entry
type Crumb struct {
	Link string
	Text string
}

// NewDirectory makes an empty Directory
func NewDirectory(dirRemote string, htmlTemplate *template.Template) *Directory {
	var breadcrumb []Crumb

	// skip trailing slash
	lpath := "/" + dirRemote
	if lpath[len(lpath)-1] == '/' {
		lpath = lpath[:len(lpath)-1]
	}

	parts := strings.Split(lpath, "/")
	for i := range parts {
		txt := parts[i]
		if i == 0 && parts[i] == "" {
			txt = "/"
		}
		lnk := strings.Repeat("../", len(parts)-i-1)
		breadcrumb = append(breadcrumb, Crumb{Link: lnk, Text: txt})
	}

	d := &Directory{
		DirRemote:    dirRemote,
		Title:        fmt.Sprintf("Directory listing of /%s", dirRemote),
		Name:         fmt.Sprintf("/%s", dirRemote),
		ZipURL:       "?download=zip",
		HTMLTemplate: htmlTemplate,
		Breadcrumb:   breadcrumb,
	}
	return d
}

// SetLinkIndex makes directory links point at name inside the
// directory, e.g. "dir/index.html" rather than "dir/", for hosts which
// don't serve index documents. Call it before adding entries.
func (d *Directory) SetLinkIndex(name string) *Directory {
	d.LinkIndex = name
	for i := range d.Breadcrumb {
		d.Breadcrumb[i].Link += name
	}
	return d
}

// Path returns the path of the directory from the root with leading
// and trailing slashes, e.g. "/dir/" or "/" for the root.
func (d *Directory) Path() string {
	if d.IsRoot() {
		return "/"
	}
	return "/" + strings.Trim(d.DirRemote, "/") + "/"
}

// IsRoot returns true if this is the root directory
func (d *Directory) IsRoot() bool {
	return strings.Trim(d.DirRemote, "/") == ""
}

// UpLink returns the link to the parent directory
func (d *Directory) UpLink() string {
	if d.LinkIndex == "" {
		return ".."
	}
	return "../" + d.LinkIndex
}

// NumDirs returns the number of directory entries
func (d *Directory) NumDirs() (n int) {
	for i := range d.Entries {
		if d.Entries[i].IsDir {
			n++
		}
	}
	return n
}

// NumFiles returns the number of file entries
func (d *Directory) NumFiles() int {
	return len(d.Entries) - d.NumDirs()
}

// TotalSize returns the total size of the file entries
func (d *Directory) TotalSize() (size int64) {
	for i := range d.Entries {
		if !d.Entries[i].IsDir {
			size += d.Entries[i].Size
		}
	}
	return size
}

// SetQuery sets the query parameters for each URL
func (d *Directory) SetQuery(queryParams url.Values) *Directory {
	d.Query = ""
	if len(queryParams) > 0 {
		d.Query = "?" + queryParams.Encode()
	}
	return d
}

// AddHTMLEntry adds an entry to that directory
func (d *Directory) AddHTMLEntry(remote string, isDir bool, size int64, modTime time.Time) {
	leaf := path.Base(remote)
	if leaf == "." {
		leaf = ""
	}
	urlRemote := leaf
	if leaf != "" {
		// Link with a leading ./ as Caddy's file server does, since
		// released versions of rclone selfupdate look for
		// href="./vX.Y.Z/" in the listing of downloads.rclone.org
		urlRemote = "./" + leaf
	}
	mimeType := fs.MimeTypeFromName(leaf)
	linkIndex, zipURL := "", ""
	if isDir {
		leaf += "/"
		urlRemote += "/"
		mimeType = "inode/directory"
		linkIndex = d.LinkIndex
		zipURL = rest.URLPathEscape(urlRemote) + "?download=zip"
	}
	d.Entries = append(d.Entries, DirEntry{
		remote:   remote,
		URL:      rest.URLPathEscape(urlRemote) + linkIndex + d.Query,
		ZipURL:   zipURL,
		Leaf:     leaf,
		IsDir:    isDir,
		Size:     size,
		ModTime:  modTime,
		MimeType: mimeType,
	})
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

// Error logs the error and if a ResponseWriter is given it writes an http.StatusInternalServerError
func Error(ctx context.Context, what any, w http.ResponseWriter, text string, err error) {
	err = fs.CountError(ctx, err)
	fs.Errorf(what, "%s: %v", text, err)
	if w != nil {
		http.Error(w, text+".", http.StatusInternalServerError)
	}
}

// ProcessQueryParams takes and sorts/orders based on the request sort/order parameters and default is namedirfirst/asc
func (d *Directory) ProcessQueryParams(sortParm string, orderParm string) *Directory {
	d.Sort = sortParm
	d.Order = orderParm

	var toSort sort.Interface

	switch d.Sort {
	case sortByName:
		toSort = byName(*d)
	case sortByNameDirFirst:
		toSort = byNameDirFirst(*d)
	case sortBySize:
		toSort = bySize(*d)
	case sortByTime:
		toSort = byTime(*d)
	default:
		toSort = byNameDirFirst(*d)
	}
	if d.Order == "desc" && toSort != nil {
		toSort = sort.Reverse(toSort)
	}
	if toSort != nil {
		sort.Stable(toSort)
	}

	return d

}

type byName Directory
type byNameDirFirst Directory
type bySize Directory
type byTime Directory

func (d byName) Len() int      { return len(d.Entries) }
func (d byName) Swap(i, j int) { d.Entries[i], d.Entries[j] = d.Entries[j], d.Entries[i] }

func (d byName) Less(i, j int) bool {
	return strings.ToLower(d.Entries[i].Leaf) < strings.ToLower(d.Entries[j].Leaf)
}

func (d byNameDirFirst) Len() int      { return len(d.Entries) }
func (d byNameDirFirst) Swap(i, j int) { d.Entries[i], d.Entries[j] = d.Entries[j], d.Entries[i] }

func (d byNameDirFirst) Less(i, j int) bool {
	// sort by name if both are dir or file
	if d.Entries[i].IsDir == d.Entries[j].IsDir {
		return strings.ToLower(d.Entries[i].Leaf) < strings.ToLower(d.Entries[j].Leaf)
	}
	// sort dir ahead of file
	return d.Entries[i].IsDir
}

func (d bySize) Len() int      { return len(d.Entries) }
func (d bySize) Swap(i, j int) { d.Entries[i], d.Entries[j] = d.Entries[j], d.Entries[i] }

func (d bySize) Less(i, j int) bool {
	const directoryOffset = -1 << 31 // = -math.MinInt32

	iSize, jSize := d.Entries[i].Size, d.Entries[j].Size

	// directory sizes depend on the file system; to
	// provide a consistent experience, put them up front
	// and sort them by name
	if d.Entries[i].IsDir {
		iSize = directoryOffset
	}
	if d.Entries[j].IsDir {
		jSize = directoryOffset
	}
	if d.Entries[i].IsDir && d.Entries[j].IsDir {
		return strings.ToLower(d.Entries[i].Leaf) < strings.ToLower(d.Entries[j].Leaf)
	}

	return iSize < jSize
}

func (d byTime) Len() int           { return len(d.Entries) }
func (d byTime) Swap(i, j int)      { d.Entries[i], d.Entries[j] = d.Entries[j], d.Entries[i] }
func (d byTime) Less(i, j int) bool { return d.Entries[i].ModTime.Before(d.Entries[j].ModTime) }

const (
	sortByName         = "name"
	sortByNameDirFirst = "namedirfirst"
	sortBySize         = "size"
	sortByTime         = "time"
)

// Render writes the directory listing to w using the HTML template
func (d *Directory) Render(w io.Writer) error {
	return d.HTMLTemplate.Execute(w, d)
}

// Serve serves a directory
func (d *Directory) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Account the transfer
	tr := accounting.Stats(r.Context()).NewTransferRemoteSize(d.DirRemote, -1, nil, nil)
	defer tr.Done(r.Context(), nil)

	fs.Infof(d.DirRemote, "%s: Serving directory", r.RemoteAddr)

	buf := &bytes.Buffer{}
	err := d.Render(buf)
	if err != nil {
		Error(ctx, d.DirRemote, w, "Failed to render template", err)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	_, err = buf.WriteTo(w)
	if err != nil {
		Error(ctx, d.DirRemote, nil, "Failed to drain template buffer", err)
	}
}
