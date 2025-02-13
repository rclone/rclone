package adrive

import (
	"context"
	"io"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
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

// Fs represents a remote box
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

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{}

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	return f, err
}

// Features implements fs.Fs.
func (f *Fs) Features() *fs.Features {
	panic("unimplemented")
}

// Hashes implements fs.Fs.
func (f *Fs) Hashes() hash.Set {
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

// Name implements fs.Fs.
func (f *Fs) Name() string {
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

// Root implements fs.Fs.
func (f *Fs) Root() string {
	panic("unimplemented")
}

// String implements fs.Fs.
func (f *Fs) String() string {
	panic("unimplemented")
}
