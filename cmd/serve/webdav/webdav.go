// Package webdav implements a WebDAV server backed by rclone VFS
package webdav

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/flags"
	"github.com/rclone/rclone/fs/hash"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/http/serve"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/rclone/rclone/vfs/vfsflags"
	"github.com/spf13/cobra"
	"golang.org/x/net/webdav"
)

// Options required for http server
type Options struct {
	Auth          libhttp.AuthConfig
	HTTP          libhttp.Config
	Template      libhttp.TemplateConfig
	HashName      string
	HashType      hash.Type
	DisableGETDir bool
}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	Auth:          libhttp.DefaultAuthCfg(),
	HTTP:          libhttp.DefaultCfg(),
	Template:      libhttp.DefaultTemplateCfg(),
	HashType:      hash.None,
	DisableGETDir: false,
}

// Opt is options set by command line flags
var Opt = DefaultOpt

// flagPrefix is the prefix used to uniquely identify command line flags.
// It is intentionally empty for this package.
const flagPrefix = ""

func init() {
	flagSet := Command.Flags()
	libhttp.AddAuthFlagsPrefix(flagSet, flagPrefix, &Opt.Auth)
	libhttp.AddHTTPFlagsPrefix(flagSet, flagPrefix, &Opt.HTTP)
	libhttp.AddTemplateFlagsPrefix(flagSet, "", &Opt.Template)
	vfsflags.AddFlags(flagSet)
	proxyflags.AddFlags(flagSet)
	flags.StringVarP(flagSet, &Opt.HashName, "etag-hash", "", "", "Which hash to use for the ETag, or auto or blank for off", "")
	flags.BoolVarP(flagSet, &Opt.DisableGETDir, "disable-dir-list", "", false, "Disable HTML directory list on GET request for a directory", "")
}

// Command definition for cobra
var Command = &cobra.Command{
	Use:   "webdav remote:path",
	Short: `Serve remote:path over WebDAV.`,
	Long: `Run a basic WebDAV server to serve a remote over HTTP via the
WebDAV protocol. This can be viewed with a WebDAV client, through a web
browser, or you can make a remote of type WebDAV to read and write it.

### WebDAV options

#### --etag-hash 

This controls the ETag header.  Without this flag the ETag will be
based on the ModTime and Size of the object.

If this flag is set to "auto" then rclone will choose the first
supported hash on the backend or you can use a named hash such as
"MD5" or "SHA-1". Use the [hashsum](/commands/rclone_hashsum/) command
to see the full list.

### Access WebDAV on Windows

WebDAV shared folder can be mapped as a drive on Windows, however the default settings prevent it.
Windows will fail to connect to the server using insecure Basic authentication.
It will not even display any login dialog. Windows requires SSL / HTTPS connection to be used with Basic.
If you try to connect via Add Network Location Wizard you will get the following error:
"The folder you entered does not appear to be valid. Please choose another".
However, you still can connect if you set the following registry key on a client machine:
HKEY_LOCAL_MACHINE\SYSTEM\CurrentControlSet\Services\WebClient\Parameters\BasicAuthLevel to 2.
The BasicAuthLevel can be set to the following values:
    0 - Basic authentication disabled
    1 - Basic authentication enabled for SSL connections only
    2 - Basic authentication enabled for SSL connections and for non-SSL connections
If required, increase the FileSizeLimitInBytes to a higher value.
Navigate to the Services interface, then restart the WebClient service.

### Access Office applications on WebDAV

Navigate to following registry HKEY_CURRENT_USER\Software\Microsoft\Office\[14.0/15.0/16.0]\Common\Internet
Create a new DWORD BasicAuthLevel with value 2.
    0 - Basic authentication disabled
    1 - Basic authentication enabled for SSL connections only
    2 - Basic authentication enabled for SSL and for non-SSL connections

https://learn.microsoft.com/en-us/office/troubleshoot/powerpoint/office-opens-blank-from-sharepoint

### Serving over a unix socket

You can serve the webdav on a unix socket like this:

    rclone serve webdav --addr unix:///tmp/my.socket remote:path

and connect to it like this using rclone and the webdav backend:

    rclone --webdav-unix-socket /tmp/my.socket --webdav-url http://localhost lsf :webdav:

Note that there is no authentication on http protocol - this is expected to be
done by the permissions on the socket.

` + libhttp.Help(flagPrefix) + libhttp.TemplateHelp(flagPrefix) + libhttp.AuthHelp(flagPrefix) + vfs.Help() + proxy.Help,
	Annotations: map[string]string{
		"versionIntroduced": "v1.39",
		"groups":            "Filter",
	},
	RunE: func(command *cobra.Command, args []string) error {
		var f fs.Fs
		if proxyflags.Opt.AuthProxy == "" {
			cmd.CheckArgs(1, 1, command, args)
			f = cmd.NewFsSrc(args)
		} else {
			cmd.CheckArgs(0, 0, command, args)
		}
		Opt.HashType = hash.None
		if Opt.HashName == "auto" {
			Opt.HashType = f.Hashes().GetOne()
		} else if Opt.HashName != "" {
			err := Opt.HashType.Set(Opt.HashName)
			if err != nil {
				return err
			}
		}
		if Opt.HashType != hash.None {
			fs.Debugf(f, "Using hash %v for ETag", Opt.HashType)
		}
		cmd.Run(false, false, command, func() error {
			s, err := newWebDAV(context.Background(), f, &Opt)
			if err != nil {
				return err
			}
			err = s.serve()
			if err != nil {
				return err
			}
			defer systemd.Notify()()
			s.Wait()
			return nil
		})
		return nil
	},
}

// WebDAV is a webdav.FileSystem interface
//
// A FileSystem implements access to a collection of named files. The elements
// in a file path are separated by slash ('/', U+002F) characters, regardless
// of host operating system convention.
//
// Each method has the same semantics as the os package's function of the same
// name.
//
// Note that the os.Rename documentation says that "OS-specific restrictions
// might apply". In particular, whether or not renaming a file or directory
// overwriting another existing file or directory is an error is OS-dependent.
type WebDAV struct {
	*libhttp.Server
	opt           Options
	f             fs.Fs
	_vfs          *vfs.VFS // don't use directly, use getVFS
	webdavhandler *webdav.Handler
	proxy         *proxy.Proxy
	ctx           context.Context // for global config
}

// check interface
var _ webdav.FileSystem = (*WebDAV)(nil)

// Make a new WebDAV to serve the remote
func newWebDAV(ctx context.Context, f fs.Fs, opt *Options) (w *WebDAV, err error) {
	w = &WebDAV{
		f:   f,
		ctx: ctx,
		opt: *opt,
	}
	if proxyflags.Opt.AuthProxy != "" {
		w.proxy = proxy.New(ctx, &proxyflags.Opt)
		// override auth
		w.opt.Auth.CustomAuthFn = w.auth
	} else {
		w._vfs = vfs.New(f, &vfscommon.Opt)
	}

	w.Server, err = libhttp.NewServer(ctx,
		libhttp.WithConfig(w.opt.HTTP),
		libhttp.WithAuth(w.opt.Auth),
		libhttp.WithTemplate(w.opt.Template),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to init server: %w", err)
	}

	// Make sure BaseURL starts with a / and doesn't end with one
	w.opt.HTTP.BaseURL = "/" + strings.Trim(w.opt.HTTP.BaseURL, "/")

	webdavHandler := &webdav.Handler{
		Prefix:     w.opt.HTTP.BaseURL,
		FileSystem: w,
		LockSystem: webdav.NewMemLS(),
		Logger:     w.logRequest, // FIXME
	}
	w.webdavhandler = webdavHandler

	router := w.Server.Router()
	router.Use(
		middleware.SetHeader("Accept-Ranges", "bytes"),
		middleware.SetHeader("Server", "rclone/"+fs.Version),
	)

	router.Handle("/*", w)

	// Webdav only methods not defined in chi
	methods := []string{
		"COPY",      // Copies the resource.
		"LOCK",      // Locks the resource.
		"MKCOL",     // Creates the collection specified.
		"MOVE",      // Moves the resource.
		"PROPFIND",  // Performs a property find on the server.
		"PROPPATCH", // Sets or removes properties on the server.
		"UNLOCK",    // Unlocks the resource.
	}
	for _, method := range methods {
		chi.RegisterMethod(method)
		router.Method(method, "/*", w)
	}

	return w, nil
}

// Gets the VFS in use for this request
func (w *WebDAV) getVFS(ctx context.Context) (VFS *vfs.VFS, err error) {
	if w._vfs != nil {
		return w._vfs, nil
	}
	value := libhttp.CtxGetAuth(ctx)
	if value == nil {
		return nil, errors.New("no VFS found in context")
	}
	VFS, ok := value.(*vfs.VFS)
	if !ok {
		return nil, fmt.Errorf("context value is not VFS: %#v", value)
	}
	return VFS, nil
}

// auth does proxy authorization
func (w *WebDAV) auth(user, pass string) (value interface{}, err error) {
	VFS, _, err := w.proxy.Call(user, pass, false)
	if err != nil {
		return nil, err
	}
	return VFS, err
}

type webdavRW struct {
	http.ResponseWriter
	status int
}

func (rw *webdavRW) WriteHeader(statusCode int) {
	rw.status = statusCode
	rw.ResponseWriter.WriteHeader(statusCode)
}

func (rw *webdavRW) isSuccessfull() bool {
	return rw.status == 0 || (rw.status >= 200 && rw.status <= 299)
}

func (w *WebDAV) postprocess(r *http.Request, remote string) {
	// set modtime from requests, don't write to client because status is already written
	switch r.Method {
	case "COPY", "MOVE", "PUT":
		VFS, err := w.getVFS(r.Context())
		if err != nil {
			fs.Errorf(nil, "Failed to get VFS: %v", err)
			return
		}

		// Get the node
		node, err := VFS.Stat(remote)
		if err != nil {
			fs.Errorf(nil, "Failed to stat node: %v", err)
			return
		}

		mh := r.Header.Get("X-OC-Mtime")
		if mh != "" {
			modtimeUnix, err := strconv.ParseInt(mh, 10, 64)
			if err == nil {
				err = node.SetModTime(time.Unix(modtimeUnix, 0))
				if err != nil {
					fs.Errorf(nil, "Failed to set modtime: %v", err)
				}
			} else {
				fs.Errorf(nil, "Failed to parse modtime: %v", err)
			}
		}
	}
}

func (w *WebDAV) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	urlPath := r.URL.Path
	isDir := strings.HasSuffix(urlPath, "/")
	remote := strings.Trim(urlPath, "/")
	if !w.opt.DisableGETDir && (r.Method == "GET" || r.Method == "HEAD") && isDir {
		w.serveDir(rw, r, remote)
		return
	}
	// Add URL Prefix back to path since webdavhandler needs to
	// return absolute references.
	r.URL.Path = w.opt.HTTP.BaseURL + r.URL.Path
	wrw := &webdavRW{ResponseWriter: rw}
	w.webdavhandler.ServeHTTP(wrw, r)

	if wrw.isSuccessfull() {
		w.postprocess(r, remote)
	}
}

// serveDir serves a directory index at dirRemote
// This is similar to serveDir in serve http.
func (w *WebDAV) serveDir(rw http.ResponseWriter, r *http.Request, dirRemote string) {
	VFS, err := w.getVFS(r.Context())
	if err != nil {
		http.Error(rw, "Root directory not found", http.StatusNotFound)
		fs.Errorf(nil, "Failed to serve directory: %v", err)
		return
	}
	// List the directory
	node, err := VFS.Stat(dirRemote)
	if err == vfs.ENOENT {
		http.Error(rw, "Directory not found", http.StatusNotFound)
		return
	} else if err != nil {
		serve.Error(dirRemote, rw, "Failed to list directory", err)
		return
	}
	if !node.IsDir() {
		http.Error(rw, "Not a directory", http.StatusNotFound)
		return
	}
	dir := node.(*vfs.Dir)
	dirEntries, err := dir.ReadDirAll()

	if err != nil {
		serve.Error(dirRemote, rw, "Failed to list directory", err)
		return
	}

	// Make the entries for display
	directory := serve.NewDirectory(dirRemote, w.Server.HTMLTemplate())
	for _, node := range dirEntries {
		if vfscommon.Opt.NoModTime {
			directory.AddHTMLEntry(node.Path(), node.IsDir(), node.Size(), time.Time{})
		} else {
			directory.AddHTMLEntry(node.Path(), node.IsDir(), node.Size(), node.ModTime().UTC())
		}
	}

	sortParm := r.URL.Query().Get("sort")
	orderParm := r.URL.Query().Get("order")
	directory.ProcessQueryParams(sortParm, orderParm)

	directory.Serve(rw, r)
}

// serve runs the http server in the background.
//
// Use s.Close() and s.Wait() to shutdown server
func (w *WebDAV) serve() error {
	w.Serve()
	fs.Logf(w.f, "WebDav Server started on %s", w.URLs())
	return nil
}

// logRequest is called by the webdav module on every request
func (w *WebDAV) logRequest(r *http.Request, err error) {
	fs.Infof(r.URL.Path, "%s from %s", r.Method, r.RemoteAddr)
}

// Mkdir creates a directory
func (w *WebDAV) Mkdir(ctx context.Context, name string, perm os.FileMode) (err error) {
	// defer log.Trace(name, "perm=%v", perm)("err = %v", &err)
	VFS, err := w.getVFS(ctx)
	if err != nil {
		return err
	}
	dir, leaf, err := VFS.StatParent(name)
	if err != nil {
		return err
	}
	_, err = dir.Mkdir(leaf)
	return err
}

// OpenFile opens a file or a directory
func (w *WebDAV) OpenFile(ctx context.Context, name string, flags int, perm os.FileMode) (file webdav.File, err error) {
	// defer log.Trace(name, "flags=%v, perm=%v", flags, perm)("err = %v", &err)
	VFS, err := w.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	f, err := VFS.OpenFile(name, flags, perm)
	if err != nil {
		return nil, err
	}
	return Handle{Handle: f, w: w, ctx: ctx}, nil
}

// RemoveAll removes a file or a directory and its contents
func (w *WebDAV) RemoveAll(ctx context.Context, name string) (err error) {
	// defer log.Trace(name, "")("err = %v", &err)
	VFS, err := w.getVFS(ctx)
	if err != nil {
		return err
	}
	node, err := VFS.Stat(name)
	if err != nil {
		return err
	}
	err = node.RemoveAll()
	if err != nil {
		return err
	}
	return nil
}

// Rename a file or a directory
func (w *WebDAV) Rename(ctx context.Context, oldName, newName string) (err error) {
	// defer log.Trace(oldName, "newName=%q", newName)("err = %v", &err)
	VFS, err := w.getVFS(ctx)
	if err != nil {
		return err
	}
	return VFS.Rename(oldName, newName)
}

// Stat returns info about the file or directory
func (w *WebDAV) Stat(ctx context.Context, name string) (fi os.FileInfo, err error) {
	// defer log.Trace(name, "")("fi=%+v, err = %v", &fi, &err)
	VFS, err := w.getVFS(ctx)
	if err != nil {
		return nil, err
	}
	fi, err = VFS.Stat(name)
	if err != nil {
		return nil, err
	}
	return FileInfo{FileInfo: fi, w: w}, nil
}

// Handle represents an open file
type Handle struct {
	vfs.Handle
	w   *WebDAV
	ctx context.Context
}

// Readdir reads directory entries from the handle
func (h Handle) Readdir(count int) (fis []os.FileInfo, err error) {
	fis, err = h.Handle.Readdir(count)
	if err != nil {
		return nil, err
	}
	// Wrap each FileInfo
	for i := range fis {
		fis[i] = FileInfo{FileInfo: fis[i], w: h.w}
	}
	return fis, nil
}

// Stat the handle
func (h Handle) Stat() (fi os.FileInfo, err error) {
	fi, err = h.Handle.Stat()
	if err != nil {
		return nil, err
	}
	return FileInfo{FileInfo: fi, w: h.w}, nil
}

// DeadProps returns extra properties about the handle
func (h Handle) DeadProps() (map[xml.Name]webdav.Property, error) {
	var (
		xmlName    xml.Name
		property   webdav.Property
		properties = make(map[xml.Name]webdav.Property)
	)
	if h.w.opt.HashType != hash.None {
		entry := h.Handle.Node().DirEntry()
		if o, ok := entry.(fs.Object); ok {
			hash, err := o.Hash(h.ctx, h.w.opt.HashType)
			if err == nil {
				xmlName.Space = "http://owncloud.org/ns"
				xmlName.Local = "checksums"
				property.XMLName = xmlName
				property.InnerXML = append(property.InnerXML, "<checksum xmlns=\"http://owncloud.org/ns\">"...)
				property.InnerXML = append(property.InnerXML, strings.ToUpper(h.w.opt.HashType.String())...)
				property.InnerXML = append(property.InnerXML, ':')
				property.InnerXML = append(property.InnerXML, hash...)
				property.InnerXML = append(property.InnerXML, "</checksum>"...)
				properties[xmlName] = property
			} else {
				fs.Errorf(nil, "failed to calculate hash: %v", err)
			}
		}
	}

	xmlName.Space = "DAV:"
	xmlName.Local = "lastmodified"
	property.XMLName = xmlName
	property.InnerXML = strconv.AppendInt(nil, h.Handle.Node().ModTime().Unix(), 10)
	properties[xmlName] = property

	return properties, nil
}

// Patch changes modtime of the underlying resources, it returns ok for all properties, the error is from setModtime if any
// FIXME does not check for invalid property and SetModTime error
func (h Handle) Patch(proppatches []webdav.Proppatch) ([]webdav.Propstat, error) {
	var (
		stat webdav.Propstat
		err  error
	)
	stat.Status = http.StatusOK
	for _, patch := range proppatches {
		for _, prop := range patch.Props {
			stat.Props = append(stat.Props, webdav.Property{XMLName: prop.XMLName})
			if prop.XMLName.Space == "DAV:" && prop.XMLName.Local == "lastmodified" {
				var modtimeUnix int64
				modtimeUnix, err = strconv.ParseInt(string(prop.InnerXML), 10, 64)
				if err == nil {
					err = h.Handle.Node().SetModTime(time.Unix(modtimeUnix, 0))
				}
			}
		}
	}
	return []webdav.Propstat{stat}, err
}

// FileInfo represents info about a file satisfying os.FileInfo and
// also some additional interfaces for webdav for ETag and ContentType
type FileInfo struct {
	os.FileInfo
	w *WebDAV
}

// ETag returns an ETag for the FileInfo
func (fi FileInfo) ETag(ctx context.Context) (etag string, err error) {
	// defer log.Trace(fi, "")("etag=%q, err=%v", &etag, &err)
	if fi.w.opt.HashType == hash.None {
		return "", webdav.ErrNotImplemented
	}
	node, ok := (fi.FileInfo).(vfs.Node)
	if !ok {
		fs.Errorf(fi, "Expecting vfs.Node, got %T", fi.FileInfo)
		return "", webdav.ErrNotImplemented
	}
	entry := node.DirEntry()
	o, ok := entry.(fs.Object)
	if !ok {
		return "", webdav.ErrNotImplemented
	}
	hash, err := o.Hash(ctx, fi.w.opt.HashType)
	if err != nil || hash == "" {
		return "", webdav.ErrNotImplemented
	}
	return `"` + hash + `"`, nil
}

// ContentType returns a content type for the FileInfo
func (fi FileInfo) ContentType(ctx context.Context) (contentType string, err error) {
	// defer log.Trace(fi, "")("etag=%q, err=%v", &contentType, &err)
	node, ok := (fi.FileInfo).(vfs.Node)
	if !ok {
		fs.Errorf(fi, "Expecting vfs.Node, got %T", fi.FileInfo)
		return "application/octet-stream", nil
	}
	entry := node.DirEntry() // can be nil
	switch x := entry.(type) {
	case fs.Object:
		return fs.MimeType(ctx, x), nil
	case fs.Directory:
		return "inode/directory", nil
	case nil:
		return mime.TypeByExtension(path.Ext(node.Name())), nil
	}
	fs.Errorf(fi, "Expecting fs.Object or fs.Directory, got %T", entry)
	return "application/octet-stream", nil
}
