package serve

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"html/template"
	"math"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/lib/rest"
)

// DirEntry is a directory entry
type DirEntry struct {
	remote  string
	URL     string
	ZipURL  string
	Leaf    string
	IsDir   bool
	Size    int64
	ModTime time.Time
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
	if isDir {
		leaf += "/"
		urlRemote += "/"
	}
	d.Entries = append(d.Entries, DirEntry{
		remote:  remote,
		URL:     rest.URLPathEscape(urlRemote) + d.Query,
		ZipURL:  "",
		Leaf:    leaf,
		IsDir:   isDir,
		Size:    size,
		ModTime: modTime,
	})
	if isDir {
		d.Entries[len(d.Entries)-1].ZipURL = rest.URLPathEscape(urlRemote) + "?download=zip"
	}
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

	var sortFn func(a, b DirEntry) int
	switch d.Sort {
	case sortByName:
		sortFn = sortDirEntryByName
	case sortByNameDirFirst:
		sortFn = sortDirEntryByNameDirFirst
	case sortBySize:
		sortFn = sortDirEntryBySize
	case sortByTime:
		sortFn = sortDirEntryByTime
	default:
		sortFn = sortDirEntryByNameDirFirst
	}
	slices.SortFunc(d.Entries, sortFn)

	if d.Order == "desc" {
		slices.Reverse(d.Entries)
	}

	return d
}

func sortDirEntryByName(a, b DirEntry) int {
	aLeaf := strings.ToLower(a.Leaf)
	bLeaf := strings.ToLower(b.Leaf)

	// trim trailing '/' because it's not part of a filename. This also ensures
	// correct dir order - 'test/' should be placed before 'test 1/'
	if a.IsDir {
		aLeaf = strings.TrimSuffix(aLeaf, "/")
	}
	if b.IsDir {
		bLeaf = strings.TrimSuffix(bLeaf, "/")
	}

	return cmp.Compare(aLeaf, bLeaf)
}

func sortDirEntryByNameDirFirst(a, b DirEntry) int {
	// sort by name if both are dir or file
	if a.IsDir == b.IsDir {
		return sortDirEntryByName(a, b)
	}
	// sort dir ahead of file
	if a.IsDir {
		return -1
	}
	return +1
}

func sortDirEntryBySize(a, b DirEntry) int {
	const directoryOffset = math.MinInt32

	iSize, jSize := a.Size, b.Size

	// directory sizes depend on the file system; to
	// provide a consistent experience, put them up front
	// and sort them by name
	if a.IsDir {
		iSize = directoryOffset
	}
	if b.IsDir {
		jSize = directoryOffset
	}
	if a.IsDir && b.IsDir {
		return sortDirEntryByName(a, b)
	}

	if v := cmp.Compare(iSize, jSize); v != 0 {
		return v
	}
	return sortDirEntryByName(a, b)
}

func sortDirEntryByTime(a, b DirEntry) int {
	if v := a.ModTime.Compare(b.ModTime); v != 0 {
		return v
	}
	return sortDirEntryByNameDirFirst(a, b)
}

const (
	sortByName         = "name"
	sortByNameDirFirst = "namedirfirst"
	sortBySize         = "size"
	sortByTime         = "time"
)

// Serve serves a directory
func (d *Directory) Serve(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Account the transfer
	tr := accounting.Stats(r.Context()).NewTransferRemoteSize(d.DirRemote, -1, nil, nil)
	defer tr.Done(r.Context(), nil)

	fs.Infof(d.DirRemote, "%s: Serving directory", r.RemoteAddr)

	buf := &bytes.Buffer{}
	err := d.HTMLTemplate.Execute(buf, d)
	if err != nil {
		Error(ctx, d.DirRemote, w, "Failed to render template", err)
		return
	}
	w.Header().Set("Content-Length", fmt.Sprintf("%d", buf.Len()))
	_, err = buf.WriteTo(w)
	if err != nil {
		Error(ctx, d.DirRemote, nil, "Failed to drain template buffer", err)
	}
}
