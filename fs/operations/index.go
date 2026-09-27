package operations

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	htmltemplate "html/template"
	"io"
	"os"
	"path"
	"sort"
	"strings"
	texttemplate "text/template"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/errcount"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/serve"
	"golang.org/x/sync/errgroup"
)

type indexDirTimeChoices struct{}

func (indexDirTimeChoices) Choices() []string {
	return []string{
		IndexDirTimeNewest: "newest",
		IndexDirTimeDir:    "dir",
		IndexDirTimeNone:   "none",
	}
}

// IndexDirTime describes how Index works out the time shown for a directory
type IndexDirTime = fs.Enum[indexDirTimeChoices]

// IndexDirTime constants
const (
	IndexDirTimeNewest IndexDirTime = iota // the newest modification time of anything below the directory
	IndexDirTimeDir                        // the directory's own modification time
	IndexDirTimeNone                       // no time
)

// IndexOpt are the options for Index
type IndexOpt struct {
	Outputs   []string        `json:"output"`    // listings to write in each directory as NAME=FORMAT
	Rules     filter.RulesOpt `json:"rules"`     // which directories get listings
	MaxDepth  int             `json:"maxDepth"`  // only make listings this many directories deep, -1 for no limit
	LinkIndex bool            `json:"linkIndex"` // link to dir/NAME rather than dir/
	DirTime   IndexDirTime    `json:"dirTime"`   // how to work out directory times
	NoModTime bool            `json:"noModTime"` // don't show modification times
	Rewrite   bool            `json:"rewrite"`   // write every listing even if it is unchanged
}

// IndexOptDefault are the default options for Index
var IndexOptDefault = IndexOpt{
	Outputs:  []string{"index.html=html"},
	MaxDepth: -1,
}

// indexFormats are the built-in output formats
var indexFormats = map[string]struct{ file, mimeType string }{
	"html":  {"templates/index.html", "text/html; charset=utf-8"},
	"json":  {"templates/index.json", "application/json"},
	"caddy": {"templates/caddy.json", "application/json"},
}

// IndexTemplate returns the built-in template for format
func IndexTemplate(format string) (string, error) {
	f, ok := indexFormats[format]
	if !ok {
		return "", fmt.Errorf("unknown built-in format %q", format)
	}
	data, err := libhttp.Assets.ReadFile(f.file)
	return string(data), err
}

// indexTemplate is implemented by both html/template and text/template
type indexTemplate interface {
	Execute(w io.Writer, data any) error
}

// indexOutput is one listing to write in each directory
type indexOutput struct {
	name     string
	mimeType string
	tmpl     indexTemplate
}

// indexDir is what Index knows about a directory
type indexDir struct {
	remote     string
	depth      int
	files      []fs.Object          // files to list, after filtering, excluding outputs
	subdirs    []fs.Directory       // directories to list, after filtering
	outputs    map[string]fs.Object // existing outputs by leaf name
	excluded   bool                 // excluded by the index rules so nothing is written or deleted here
	hasContent bool                 // there is something here other than outputs, before filtering
	gone       bool                 // contains nothing but outputs so will cease to exist
	newest     time.Time            // the newest modification time of anything below here
}

// index holds the state of one run of Index
type index struct {
	f            fs.Fs
	opt          *IndexOpt
	fi           *filter.Filter
	includeDir   func(string) (bool, error) // the normal filters applied to a directory
	indexDir     func(string) (bool, error) // the index rules applied to a directory
	outputs      []indexOutput
	isOutput     map[string]bool
	htmlTemplate *htmltemplate.Template
	dirTime      IndexDirTime
	now          time.Time
	dirs         map[string]*indexDir
	transfers    errgroup.Group
	ec           *errcount.ErrCount
}

// Index writes a directory listing into every directory of f so that
// it can be browsed when served as a static website.
//
// It works like sync: existing listings which are unchanged are left
// alone, changed ones are rewritten, and listings in directories
// which contain nothing else are deleted on backends which can't have
// empty directories.
func Index(ctx context.Context, f fs.Fs, opt *IndexOpt) error {
	ix, err := newIndex(ctx, f, opt)
	if err != nil {
		return err
	}
	return ix.run(ctx)
}

// newIndex parses the options and templates
func newIndex(ctx context.Context, f fs.Fs, opt *IndexOpt) (*index, error) {
	ix := &index{
		f:        f,
		opt:      opt,
		fi:       filter.GetConfig(ctx),
		isOutput: map[string]bool{},
		dirTime:  opt.DirTime,
		now:      time.Now(),
		dirs:     map[string]*indexDir{},
		ec:       errcount.New(),
	}
	if ix.opt.NoModTime {
		ix.dirTime = IndexDirTimeNone
	}
	ix.includeDir = ix.fi.IncludeDirectory(ctx, f)

	// The index rules are a second rule set applied to directory paths
	indexFilter, err := filter.NewFilter(&filter.Options{
		RulesOpt: opt.Rules,
		MinAge:   fs.DurationOff,
		MaxAge:   fs.DurationOff,
		MinSize:  -1,
		MaxSize:  -1,
	})
	if err != nil {
		return nil, fmt.Errorf("index rules: %w", err)
	}
	ix.indexDir = indexFilter.IncludeDirectory(ctx, nil)

	ix.htmlTemplate, err = libhttp.GetTemplate("")
	if err != nil {
		return nil, err
	}
	if len(opt.Outputs) == 0 {
		return nil, errors.New("need at least one output")
	}
	for _, s := range opt.Outputs {
		name, format, ok := strings.Cut(s, "=")
		if !ok || name == "" || name == "." || name == ".." || format == "" || strings.Contains(name, "/") {
			return nil, fmt.Errorf("bad output %q: want NAME=FORMAT", s)
		}
		out, err := ix.newOutput(name, format)
		if err != nil {
			return nil, fmt.Errorf("output %q: %w", s, err)
		}
		ix.outputs = append(ix.outputs, out)
		ix.isOutput[name] = true
	}
	return ix, nil
}

// newOutput makes an output from a NAME=FORMAT pair, where format is
// a built-in format or the path of a template file
func (ix *index) newOutput(name, format string) (out indexOutput, err error) {
	out.name = name
	if builtin, ok := indexFormats[format]; ok {
		out.mimeType = builtin.mimeType
		if format == "html" {
			out.tmpl = ix.htmlTemplate
			return out, nil
		}
		data, err := libhttp.Assets.ReadFile(builtin.file)
		if err != nil {
			return out, err
		}
		out.tmpl, err = newIndexTextTemplate(string(data))
		return out, err
	}
	out.mimeType = fs.MimeTypeFromName(name)
	switch strings.ToLower(path.Ext(name)) {
	case ".html", ".htm":
		out.tmpl, err = libhttp.GetTemplate(format)
	default:
		var data []byte
		data, err = os.ReadFile(format)
		if err != nil {
			return out, err
		}
		out.tmpl, err = newIndexTextTemplate(string(data))
	}
	return out, err
}

// newIndexTextTemplate parses a text/template for non-HTML outputs
func newIndexTextTemplate(text string) (*texttemplate.Template, error) {
	funcMap := texttemplate.FuncMap{
		"json": func(v any) (string, error) {
			data, err := json.Marshal(v)
			return string(data), err
		},
		"afterEpoch": libhttp.AfterEpoch,
		"contains":   strings.Contains,
		"hasPrefix":  strings.HasPrefix,
		"hasSuffix":  strings.HasSuffix,
		"trimSuffix": strings.TrimSuffix,
	}
	return texttemplate.New("index").Funcs(funcMap).Parse(text)
}

// run does the work of Index
func (ix *index) run(ctx context.Context) error {
	ci := fs.GetConfig(ctx)
	err := walk.Walk(ctx, ix.f, "", true, ConfigMaxDepth(ctx, true), ix.walkFn(ctx))
	if err != nil {
		return err
	}
	ix.prune()

	// Directories deepest first so that a directory's subdirectories
	// are done before it.
	dirs := make([]*indexDir, 0, len(ix.dirs))
	for _, d := range ix.dirs {
		dirs = append(dirs, d)
	}
	sort.Slice(dirs, func(i, j int) bool {
		return dirs[i].depth > dirs[j].depth
	})

	// Read the modification times --checkers at a time as they may
	// need a transaction each on some backends, and count them as
	// checks so the progress is visible
	if !ix.opt.NoModTime {
		var g errgroup.Group
		g.SetLimit(ci.Checkers)
		for _, d := range dirs {
			for _, o := range d.files {
				g.Go(func() error {
					tr := accounting.Stats(ctx).NewCheckingTransfer(o, "reading modtime")
					o.ModTime(ctx)
					tr.Done(ctx, nil)
					return nil
				})
			}
		}
		_ = g.Wait()
		for _, d := range dirs {
			ix.setNewest(ctx, d)
		}
	}

	ix.transfers.SetLimit(ci.Transfers)
	var g errgroup.Group
	g.SetLimit(ci.Checkers)
	for _, d := range dirs {
		g.Go(func() error {
			ix.processDir(ctx, d)
			return nil
		})
	}
	_ = g.Wait()
	_ = ix.transfers.Wait()
	return ix.ec.Err("index")
}

// walkFn returns the function which collects the directories
func (ix *index) walkFn(ctx context.Context) walk.Func {
	return func(remote string, entries fs.DirEntries, err error) error {
		if err != nil {
			return err
		}
		if remote != "" {
			// Directories excluded by the normal filters aren't listed
			ok, err := ix.includeDir(remote)
			if err != nil {
				return err
			}
			if !ok {
				return walk.ErrorSkipDir
			}
		}
		// Directories excluded by the index rules are still walked
		// as they may have included directories below them and
		// their contents count towards their parent's time.
		included, err := ix.indexDir(remote)
		if err != nil {
			return err
		}
		d := &indexDir{
			remote:   remote,
			outputs:  map[string]fs.Object{},
			excluded: !included,
		}
		if remote != "" {
			d.depth = strings.Count(remote, "/") + 1
		}
		for _, entry := range entries {
			switch x := entry.(type) {
			case fs.Object:
				leaf := path.Base(x.Remote())
				if !d.excluded && ix.isOutput[leaf] {
					d.outputs[leaf] = x
					continue
				}
				d.hasContent = true
				if ix.fi.IncludeObject(ctx, x) {
					d.files = append(d.files, x)
				}
			case fs.Directory:
				ok, err := ix.includeDir(x.Remote())
				if err != nil {
					return err
				}
				if ok {
					d.subdirs = append(d.subdirs, x)
				} else {
					d.hasContent = true
				}
			}
		}
		ix.dirs[remote] = d
		return nil
	}
}

// prune works out which directories contain nothing but outputs.
//
// On backends which can't have empty directories these will cease to
// exist once their outputs are deleted, so they are removed from their
// parent's listing, which may in turn leave the parent with nothing
// but outputs.
func (ix *index) prune() {
	if ix.f.Features().CanHaveEmptyDirectories {
		return
	}
	var pruneDir func(d *indexDir)
	pruneDir = func(d *indexDir) {
		kept := d.subdirs[:0]
		for _, sub := range d.subdirs {
			if sd := ix.dirs[sub.Remote()]; sd != nil {
				pruneDir(sd)
				if sd.gone {
					continue
				}
			}
			kept = append(kept, sub)
		}
		d.subdirs = kept
		d.gone = !d.hasContent && len(d.subdirs) == 0
	}
	if root := ix.dirs[""]; root != nil {
		pruneDir(root)
	}
}

// setNewest sets d.newest from its files and subdirectories, which
// must have been done already
func (ix *index) setNewest(ctx context.Context, d *indexDir) {
	for _, o := range d.files {
		if t := o.ModTime(ctx); t.After(d.newest) {
			d.newest = t
		}
	}
	for _, sub := range d.subdirs {
		if sd := ix.dirs[sub.Remote()]; sd != nil && sd.newest.After(d.newest) {
			d.newest = sd.newest
		}
	}
}

// processDir brings the outputs of d up to date
func (ix *index) processDir(ctx context.Context, d *indexDir) {
	if d.gone {
		for _, o := range d.outputs {
			ix.transfers.Go(func() error {
				err := DeleteFile(ctx, o)
				if err != nil {
					ix.ec.Add(err)
				}
				return nil
			})
		}
		return
	}
	if d.excluded || (ix.opt.MaxDepth >= 0 && d.depth >= ix.opt.MaxDepth) {
		return
	}
	dir := ix.render(ctx, d)
	modTime := d.newest
	if modTime.IsZero() {
		modTime = ix.now
	}
	for _, out := range ix.outputs {
		var buf bytes.Buffer
		err := out.tmpl.Execute(&buf, dir)
		if err != nil {
			ix.ec.Add(err)
			fs.Errorf(d.remote, "Failed to render %s: %v", out.name, fs.CountError(ctx, err))
			continue
		}
		remote := path.Join(d.remote, out.name)
		src := object.NewMemoryObject(remote, modTime, buf.Bytes()).WithMimeType(out.mimeType)
		dst := d.outputs[out.name]
		if dst != nil && !ix.opt.Rewrite {
			equal, err := ix.equal(ctx, src, dst)
			if err != nil {
				ix.ec.Add(err)
				fs.Errorf(dst, "Failed to check listing: %v", fs.CountError(ctx, err))
				continue
			}
			if equal {
				fs.Debugf(dst, "Unchanged skipping")
				continue
			}
		}
		ix.transfers.Go(func() error {
			_, err := Copy(ctx, ix.f, dst, remote, src)
			if err != nil {
				ix.ec.Add(err)
				fs.Errorf(remote, "Failed to write listing: %v", fs.CountError(ctx, err))
			}
			return nil
		})
	}
}

// render makes the listing for d
func (ix *index) render(ctx context.Context, d *indexDir) *serve.Directory {
	dir := serve.NewDirectory(d.remote, ix.htmlTemplate)
	dir.Static = true
	dir.DisableZip = true
	if ix.opt.LinkIndex {
		dir.SetLinkIndex(ix.outputs[0].name)
	}
	for _, sub := range d.subdirs {
		var t time.Time
		switch ix.dirTime {
		case IndexDirTimeDir:
			t = sub.ModTime(ctx)
		case IndexDirTimeNewest:
			if sd := ix.dirs[sub.Remote()]; sd != nil {
				t = sd.newest
			}
		}
		// Not sub.Size() as some backends, eg local on macOS, report a
		// directory size which changes when the listings are written
		dir.AddHTMLEntry(sub.Remote(), true, -1, t.UTC())
	}
	for _, o := range d.files {
		var t time.Time
		if !ix.opt.NoModTime {
			t = o.ModTime(ctx)
		}
		dir.AddHTMLEntry(o.Remote(), false, o.Size(), t.UTC())
	}
	dir.ProcessQueryParams("", "")
	return dir
}

// equal reports whether the existing listing dst has the same
// contents as src, using the size and a hash where the backend has
// one, and otherwise by reading dst
func (ix *index) equal(ctx context.Context, src *object.MemoryObject, dst fs.Object) (bool, error) {
	tr := accounting.Stats(ctx).NewCheckingTransfer(dst, "checking")
	defer tr.Done(ctx, nil)
	if src.Size() != dst.Size() {
		return false, nil
	}
	equal, ht, err := CheckHashes(ctx, src, dst)
	if err != nil {
		return false, err
	}
	if ht != hash.None {
		return equal, nil
	}
	in, err := Open(ctx, dst)
	if err != nil {
		return false, err
	}
	data, err := io.ReadAll(in)
	if closeErr := in.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return false, err
	}
	return bytes.Equal(data, src.Content()), nil
}
