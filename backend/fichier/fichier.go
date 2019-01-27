package fichier

import (
	"bytes"
	"fmt"
	"github.com/ncw/rclone/fs"
	"github.com/ncw/rclone/fs/config/configmap"
	"github.com/ncw/rclone/fs/config/configstruct"
	"github.com/ncw/rclone/fs/hash"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "fichier",
		Description: "1Fichier",
		Config: func(name string, config configmap.Mapper) {
		},
		NewFs: NewFs,
		Options: []fs.Option{
			{
				Name:       "api_key",
				IsPassword: true,
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ApiKey string `config:"api_key"`
}

func (f *Fs) RoundTrip(request *http.Request) (response *http.Response, err error) {
	request.Header.Add("Authorization", "Bearer "+f.apiKey)

	return f.baseClient.Do(request)
}

type Fs struct {
	baseClient *http.Client
	authClient *http.Client

	apiKey string
}

func (f *Fs) Name() string {
	panic("implement me")
}

func (f *Fs) Root() string {
	panic("implement me")
}

func (f *Fs) String() string {
	panic("implement me")
}

func (f *Fs) Precision() time.Duration {
	panic("implement me")
}

func (f *Fs) Hashes() hash.Set {
	panic("implement me")
}

func (f *Fs) Features() *fs.Features {
	panic("implement me")
}

func (f *Fs) List(dir string) (entries fs.DirEntries, err error) {

	resp, err := f.authClient.Post("https://api.1fichier.com/v1/file/ls.cgi", "application/json", bytes.NewReader([]byte("{}")))

	if err != nil {
		panic(err)
	}

	body, err := ioutil.ReadAll(resp.Body)
	fmt.Println(string(body))

	if err != nil {
		panic(err)
	}

	return fs.DirEntries{
		//fs.NewDir("test", time.Now()),
	}, nil
}

func (f *Fs) NewObject(remote string) (fs.Object, error) {
	panic("implement me")
}

func (f *Fs) Put(in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	panic("implement me")
}

func (f *Fs) Mkdir(dir string) error {
	panic("implement me")
}

func (f *Fs) Rmdir(dir string) error {
	panic("implement me")
}

func NewFs(name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(config, opt)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		baseClient: &http.Client{},
		authClient: nil,
		apiKey:     opt.ApiKey,
	}

	f.authClient = &http.Client{
		Transport: f,
	}

	return f, nil
}
