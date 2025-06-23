// Package piqlconnect provides an interface to piqlConnect
package piqlconnect

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/piqlconnect/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/rest"
)

const (
	rootURL = "http://127.0.0.1:3000"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "piqlconnect",
		Description: "piqlConnect",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return nil, nil
		},
		Options: []fs.Option{
			{
				Name:      "api_key",
				Help:      "piqlConnect API key obtained from web interface",
				Required:  true,
				Sensitive: true,
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiKey string `config:"api_key"`
}

// Fs represents a remote piqlConnect package
type Fs struct {
	organisationId string
	client         *rest.Client
	packageIdMap   map[string]string
}

func (f *Fs) Name() string {
	return "hello"
}

func (f *Fs) Root() string {
	return "."
}

func (f *Fs) String() string {
	return "piqlConnect[" + f.organisationId + "]"
}

func (f *Fs) Features() *fs.Features {
	return &fs.Features{}
}

func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return nil, nil
}

type TopDirKind uint8

const (
	TopDirWorkspace TopDirKind = iota
	TopDirInProgress
	TopDirArchive
)

func (topDir TopDirKind) name() string {
	if topDir == TopDirWorkspace {
		return "Workspace"
	}
	if topDir == TopDirInProgress {
		return "In Progress"
	}
	if topDir == TopDirArchive {
		return "Archive"
	}
	panic("unreachable")
}

func (f *Fs) listPackages(ctx context.Context, topDir TopDirKind) (entries fs.DirEntries, err error) {
	values := url.Values{}
	values.Set("organisationId", f.organisationId)

	ps := []api.Package{}
	_, err = f.client.CallJSON(ctx, &rest.Opts{Path: "/api/packages", Parameters: values}, nil, &ps)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		f.packageIdMap[p.Name] = p.Id
		switch topDir {
		case TopDirWorkspace:
			if p.Status != "ACTIVE" {
				continue
			}
		case TopDirInProgress:
			if p.Status != "PENDING_PAYMENT" && p.Status != "PREPARING" && p.Status != "PROCESSING" {
				continue
			}
		case TopDirArchive:
			if p.Status != "ARCHIVED" {
				continue
			}
		}
		mtime, err := time.Parse(time.RFC3339, p.UpdatedAt)
		if err != nil {
			return nil, err
		}
		entries = append(entries, fs.NewDir(path.Join(topDir.name(), p.Name), mtime))
	}
	return entries, nil
}

func (fs *Fs) listFiles(ctx context.Context, packageName string, path []string) (entries fs.DirEntries, err error) {
	values := url.Values{}
	values.Set("organisationId", fs.organisationId)
	values.Set("packageId", fs.packageIdMap[packageName])
	files := []api.File{}
	_, err = fs.client.CallJSON(ctx, &rest.Opts{Path: "/api/files", Parameters: values}, nil, &files)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		fmt.Println(f)
		f.SetFs(fs)
		entries = append(entries, f)
	}
	return entries, nil
}

func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	if len(dir) == 0 {
		return append(
			entries,
			fs.NewDir(path.Join(dir, "Workspace"), time.Unix(0, 0)),
			fs.NewDir(path.Join(dir, "In Progress"), time.Unix(0, 0)),
			fs.NewDir(path.Join(dir, "Archive"), time.Unix(0, 0)),
		), nil
	}
	if dir == "Workspace" {
		return f.listPackages(ctx, TopDirWorkspace)
	}
	if dir == "In Progress" {
		return f.listPackages(ctx, TopDirInProgress)
	}
	if dir == "Archive" {
		return f.listPackages(ctx, TopDirArchive)
	}
	segments := strings.Split(dir, "/")
	if len(segments) >= 2 && segments[0] == "Workspace" {
		return f.listFiles(ctx, segments[1], segments[2:])
	}

	return entries, nil
}

func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, nil
}

func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return nil
}

func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

func (f *Fs) Precision() time.Duration {
	return time.Millisecond
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	httpclient := fshttp.NewClient(ctx)
	client := rest.NewClient(httpclient)
	client.SetRoot(rootURL)
	client.SetHeader("Authorization", "Bearer "+opt.ApiKey)
	resp, err := client.Call(ctx, &rest.Opts{Path: "/api/user/api-key/organisation"})
	if err != nil {
		return nil, err
	}
	organisationIdBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		organisationId: string(organisationIdBytes),
		client:         client,
		packageIdMap:   make(map[string]string),
	}
	fmt.Println(f)

	return f, nil
}
