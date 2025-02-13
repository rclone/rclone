package adrive

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = ""
	rcloneEncryptedClientSecret = ""
	minSleep                    = 10 * time.Millisecond
	maxSleep                    = 2 * time.Second
	decayConstant               = 2 // bigger for slower decay, exponential
	rootURL                     = "https://openapi.alipan.com"
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauthutil.Config{
		Scopes: []string{
			"user:base",
			"file:all:read",
			"file:all:write",
			"album:shared:read",
			"file:share:write",
		},
		AuthURL:      "https://openapi.alipan.com/oauth/authorize",
		TokenURL:     "https://openapi.alipan.com/oauth/access_token ",
		AuthStyle:    oauth2.AuthStyleInParams,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "adrive",
		Description: "Aliyun Drive",
		NewFs:       NewFs,
		Options:     []fs.Option{},
	})
}

// Options defines the configuration for this backend
type Options struct {
	UploadCutoff  fs.SizeSuffix        `config:"upload_cutoff"`
	CommitRetries int                  `config:"commit_retries"`
	Enc           encoder.MultiEncoder `config:"encoding"`
	RootFolderID  string               `config:"root_folder_id"`
	AccessToken   string               `config:"access_token"`
	ListChunk     int                  `config:"list_chunk"`
	OwnedBy       string               `config:"owned_by"`
	Impersonate   string               `config:"impersonate"`
}

// Fs represents a remote adrive
type Fs struct {
	name         string                // name of this remote
	root         string                // the path we are working on
	opt          Options               // parsed options
	features     *fs.Features          // optional features
	srv          *rest.Client          // the connection to the server
	dirCache     *dircache.DirCache    // Map of directory path to directory id
	pacer        *fs.Pacer             // pacer for API calls
	tokenRenewer *oauthutil.Renew      // renew the token on expiry
	uploadToken  *pacer.TokenDispenser // control concurrency
}

// Object describes a adrive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
	sha1        string    // SHA-1 of the object content
}

// ------------------------------------------------------------

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
	return fmt.Sprintf("adrive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.SHA1)
}

// CleanUp implements fs.CleanUpper.
func (f *Fs) CleanUp(ctx context.Context) error {
	panic("unimplemented")
}

// PublicLink implements fs.PublicLinker.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	panic("unimplemented")
}

// DirCacheFlush implements fs.DirCacheFlusher.
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// DirMove implements fs.DirMover.
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote string, dstRemote string) error {
	panic("unimplemented")
}

// Move implements fs.Mover.
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	panic("unimplemented")
}

// About implements fs.Abouter.
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	panic("unimplemented")
}

// Copy implements fs.Copier.
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	panic("unimplemented")
}

// PutStream implements fs.PutStreamer.
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	panic("unimplemented")
}

// Purge implements fs.Purger.
func (f *Fs) Purge(ctx context.Context, dir string) error {
	panic("unimplemented")
}

// List implements fs.Fs.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	panic("unimplemented")
}

// Mkdir implements fs.Fs.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	panic("unimplemented")
}

// NewObject implements fs.Fs.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	panic("unimplemented")
}

// Precision implements fs.Fs.
func (f *Fs) Precision() time.Duration {
	panic("unimplemented")
}

// Put implements fs.Fs.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	panic("unimplemented")
}

// Rmdir implements fs.Fs.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	panic("unimplemented")
}

// ------------------------------------------------------------

// ID implements fs.IDer.
func (o *Object) ID() string {
	panic("unimplemented")
}

// Fs implements fs.Object.
func (o *Object) Fs() fs.Info {
	panic("unimplemented")
}

// Hash implements fs.Object.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	panic("unimplemented")
}

// ModTime implements fs.Object.
func (o *Object) ModTime(context.Context) time.Time {
	panic("unimplemented")
}

// Open implements fs.Object.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	panic("unimplemented")
}

// Remote implements fs.Object.
func (o *Object) Remote() string {
	panic("unimplemented")
}

// Remove implements fs.Object.
func (o *Object) Remove(ctx context.Context) error {
	panic("unimplemented")
}

// SetModTime implements fs.Object.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	panic("unimplemented")
}

// Size implements fs.Object.
func (o *Object) Size() int64 {
	panic("unimplemented")
}

// Storable implements fs.Object.
func (o *Object) Storable() bool {
	panic("unimplemented")
}

// String implements fs.Object.
func (o *Object) String() string {
	panic("unimplemented")
}

// Update implements fs.Object.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	panic("unimplemented")
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	client := fshttp.NewClient(ctx)

	f := &Fs{
		name:  name,
		root:  root,
		opt:   *opt,
		srv:   rest.NewClient(client).SetRoot(rootURL),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// If using an accessToken, set the Authorization header
	if f.opt.AccessToken != "" {
		f.srv.SetHeader("Authorization", "Bearer "+f.opt.AccessToken)
	}

	return f, err
}

// parsePath parses a remote path
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutStreamer     = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.PublicLinker    = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
