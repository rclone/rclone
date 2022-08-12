package sia

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/sia/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2 // bigger for slower decay, exponential
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "sia",
		Description: "Sia Decentralized Cloud",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name: "api_url",
			Help: `Sia daemon API URL, like http://sia.daemon.host:9980.

Note that siad must run with --disable-api-security to open API port for other hosts (not recommended).
Keep default if Sia daemon runs on localhost.`,
			Default: "http://127.0.0.1:9980",
		}, {
			Name: "api_password",
			Help: `Sia Daemon API Password.

Can be found in the apipassword file located in HOME/.sia/ or in the daemon directory.`,
			IsPassword: true,
		}, {
			Name: "user_agent",
			Help: `Siad User Agent

Sia daemon requires the 'Sia-Agent' user agent by default for security`,
			Default:  "Sia-Agent",
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: encoder.EncodeInvalidUtf8 |
				encoder.EncodeCtl |
				encoder.EncodeDel |
				encoder.EncodeHashPercent |
				encoder.EncodeQuestion |
				encoder.EncodeDot |
				encoder.EncodeSlash,
		},
		}})
}

// Options defines the configuration for this backend
type Options struct {
	APIURL      string               `config:"api_url"`
	APIPassword string               `config:"api_password"`
	UserAgent   string               `config:"user_agent"`
	Enc         encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote siad
type Fs struct {
	name     string       // name of this remote
	root     string       // the path we are working on if any
	opt      Options      // parsed config options
	features *fs.Features // optional features
	srv      *rest.Client // the connection to siad
	pacer    *fs.Pacer    // pacer for API calls
}

// Object describes a Sia object
type Object struct {
	fs      *Fs
	remote  string
	modTime time.Time
	size    int64
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// ModTime is the last modified time (read-only)
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// Size is the file length
func (o *Object) Size() int64 {
	return o.size
}

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Hash is not supported
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Storable returns if this object is storable
func (o *Object) Storable() bool {
	return true
}

// SetModTime is not supported
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var optionsFixed []fs.OpenOption
	for _, opt := range options {
		if optRange, ok := opt.(*fs.RangeOption); ok {
			// Ignore range option if file is empty
			if o.Size() == 0 && optRange.Start == 0 && optRange.End > 0 {
				continue
			}
		}
		optionsFixed = append(optionsFixed, opt)
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:  "GET",
		Path:    path.Join("/renter/stream/", o.fs.root, o.fs.opt.Enc.FromStandardPath(o.remote)),
		Options: optionsFixed,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	var resp *http.Response
	opts := rest.Opts{
		Method:        "POST",
		Path:          path.Join("/renter/uploadstream/", o.fs.opt.Enc.FromStandardPath(path.Join(o.fs.root, o.remote))),
		Body:          in,
		ContentLength: &size,
		Parameters:    url.Values{},
	}
	opts.Parameters.Set("force", "true")

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})

	if err == nil {
		err = o.readMetaData(ctx)
	}

	return err
}

// Remove an object
func (o *Object) Remove(ctx context.Context) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method: "POST",
		Path:   path.Join("/renter/delete/", o.fs.opt.Enc.FromStandardPath(path.Join(o.fs.root, o.remote))),
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(resp, err)
	})

	return err
}

// sync the size and other metadata down for the object
func (o *Object) readMetaData(ctx context.Context) (err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   path.Join("/renter/file/", o.fs.opt.Enc.FromStandardPath(path.Join(o.fs.root, o.remote))),
	}

	var result api.FileResponse
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.CallJSON(ctx, &opts, nil, &result)
		return o.fs.shouldRetry(resp, err)
	})

	if err != nil {
		return err
	}

	o.size = int64(result.File.Filesize)
	o.modTime = result.File.ModTime

	return nil
}

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("Sia %s", f.opt.APIURL)
}

// Precision is unsupported because ModTime is not changeable
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes are not exposed anywhere
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features for this fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// List files and directories in a directory
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	dirPrefix := f.opt.Enc.FromStandardPath(path.Join(f.root, dir)) + "/"

	var result api.DirectoriesResponse
	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   path.Join("/renter/dir/", dirPrefix) + "/",
	}

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(resp, err)
	})

	if err != nil {
		return nil, err
	}

	for _, directory := range result.Directories {
		if directory.SiaPath+"/" == dirPrefix {
			continue
		}

		d := fs.NewDir(f.opt.Enc.ToStandardPath(strings.TrimPrefix(directory.SiaPath, f.opt.Enc.FromStandardPath(f.root)+"/")), directory.MostRecentModTime)
		entries = append(entries, d)
	}

	for _, file := range result.Files {
		o := &Object{fs: f,
			remote:  f.opt.Enc.ToStandardPath(strings.TrimPrefix(file.SiaPath, f.opt.Enc.FromStandardPath(f.root)+"/")),
			modTime: file.ModTime,
			size:    int64(file.Filesize)}
		entries = append(entries, o)
	}

	return entries, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (o fs.Object, err error) {
	obj := &Object{
		fs:     f,
		remote: remote,
	}
	err = obj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// Put the object into the remote siad via uploadstream
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		modTime: src.ModTime(ctx),
		size:    src.Size(),
	}

	err := o.Update(ctx, in, src, options...)
	if err == nil {
		return o, nil
	}

	// Cleanup stray files left after failed upload
	for i := 0; i < 5; i++ {
		cleanObj, cleanErr := f.NewObject(ctx, src.Remote())
		if cleanErr == nil {
			cleanErr = cleanObj.Remove(ctx)
		}
		if cleanErr == nil {
			break
		}
		if cleanErr != fs.ErrorObjectNotFound {
			fs.Logf(f, "%q: cleanup failed upload: %v", src.Remote(), cleanErr)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	return nil, err
}

// PutStream the object into the remote siad via uploadstream
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// Mkdir creates a directory
func (f *Fs) Mkdir(ctx context.Context, dir string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:     "POST",
		Path:       path.Join("/renter/dir/", f.opt.Enc.FromStandardPath(path.Join(f.root, dir))),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("action", "create")

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(resp, err)
	})

	if err == fs.ErrorDirExists {
		err = nil
	}

	return err
}

// Rmdir removes a directory
func (f *Fs) Rmdir(ctx context.Context, dir string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method: "GET",
		Path:   path.Join("/renter/dir/", f.opt.Enc.FromStandardPath(path.Join(f.root, dir))),
	}

	var result api.DirectoriesResponse
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(resp, err)
	})

	if len(result.Directories) == 0 {
		return fs.ErrorDirNotFound
	} else if len(result.Files) > 0 || len(result.Directories) > 1 {
		return fs.ErrorDirectoryNotEmpty
	}

	opts = rest.Opts{
		Method:     "POST",
		Path:       path.Join("/renter/dir/", f.opt.Enc.FromStandardPath(path.Join(f.root, dir))),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("action", "delete")

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(resp, err)
	})

	return err
}

// NewFs constructs an Fs from the path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	opt.APIURL = strings.TrimSuffix(opt.APIURL, "/")

	// Parse the endpoint
	u, err := url.Parse(opt.APIURL)
	if err != nil {
		return nil, err
	}

	rootIsDir := strings.HasSuffix(root, "/")
	root = strings.Trim(root, "/")

	f := &Fs{
		name: name,
		opt:  *opt,
		root: root,
	}
	f.pacer = fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant)))

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Adjust client config and pass it attached to context
	cliCtx, cliCfg := fs.AddConfig(ctx)
	if opt.UserAgent != "" {
		cliCfg.UserAgent = opt.UserAgent
	}
	f.srv = rest.NewClient(fshttp.NewClient(cliCtx))
	f.srv.SetRoot(u.String())
	f.srv.SetErrorHandler(errorHandler)

	if opt.APIPassword != "" {
		opt.APIPassword, err = obscure.Reveal(opt.APIPassword)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt API password: %w", err)
		}
		f.srv.SetUserPass("", opt.APIPassword)
	}

	if root != "" && !rootIsDir {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(ctx, remote)
		if err != nil {
			if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorNotAFile) {
				// File doesn't exist so return old f
				f.root = root
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// errorHandler translates Siad errors into native rclone filesystem errors.
// Sadly this is using string matching since Siad can't expose meaningful codes.
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error when trying to read error body: %w", err)
	}
	// Decode error response
	errResponse := new(api.Error)
	err = json.Unmarshal(body, &errResponse)
	if err != nil {
		// Set the Message to be the body if we can't parse the JSON
		errResponse.Message = strings.TrimSpace(string(body))
	}
	errResponse.Status = resp.Status
	errResponse.StatusCode = resp.StatusCode

	msg := strings.Trim(errResponse.Message, "[]")
	code := errResponse.StatusCode
	switch {
	case code == 400 && msg == "no file known with that path":
		return fs.ErrorObjectNotFound
	case code == 400 && strings.HasPrefix(msg, "unable to get the fileinfo from the filesystem") && strings.HasSuffix(msg, "path does not exist"):
		return fs.ErrorObjectNotFound
	case code == 500 && strings.HasPrefix(msg, "failed to create directory") && strings.HasSuffix(msg, "a siadir already exists at that location"):
		return fs.ErrorDirExists
	case code == 500 && strings.HasPrefix(msg, "failed to get directory contents") && strings.HasSuffix(msg, "path does not exist"):
		return fs.ErrorDirNotFound
	case code == 500 && strings.HasSuffix(msg, "no such file or directory"):
		return fs.ErrorDirNotFound
	}
	return errResponse
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(resp *http.Response, err error) (bool, error) {
	return fserrors.ShouldRetry(err), err
}
