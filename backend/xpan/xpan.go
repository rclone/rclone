package xpan

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/xpan/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	appKey             = "p0bpXvQn6AWBSCObg2bc67IZ0GIGRSme"
	secretKey          = ""
	pacerMinSleep      = 10 * time.Millisecond
	pacerMaxSleep      = 2 * time.Second
	pacerDecayConstant = 2 // bigger for slower decay, exponential
)

var (
	oauthConfig = &oauth2.Config{
		Scopes: []string{"basic", "netdisk"},
		Endpoint: oauth2.Endpoint{
			AuthURL:   "http://openapi.baidu.com/oauth/2.0/authorize",
			TokenURL:  "https://openapi.baidu.com/oauth/2.0/token",
			AuthStyle: oauth2.AuthStyleInParams,
		},
		ClientID:     appKey,
		ClientSecret: os.Getenv("XPAN_SK"),
		RedirectURL:  oauthutil.RedirectURL,
	}
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "xpan",
		Description: "Baidu NetDisk",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{
			{
				Name:     "chunk_size",
				Help:     `Chunk size to use for uploading.`,
				Default:  fs.SizeSuffix(4 * 1024 * 1024),
				Required: true,
				Advanced: true,
			}, {
				Name:     "query_per_minute",
				Help:     `Rate limit to avoid hit frequency limit.`,
				Default:  180,
				Required: true,
				Advanced: true,
			}, {
				Name:     "tmp_dir",
				Help:     `Where temporary files are stored.`,
				Default:  filepath.Join(os.TempDir(), "xpan"),
				Required: true,
				Advanced: true,
			}, {
				Name:     config.ConfigEncoding,
				Help:     config.ConfigEncodingHelp,
				Advanced: true,
				Default: (encoder.Display |
					encoder.EncodeLtGt |
					encoder.EncodePipe |
					encoder.EncodeAsterisk |
					encoder.EncodeQuestion |
					encoder.EncodeSlash |
					encoder.EncodeDoubleQuote |
					encoder.EncodeSingleQuote |
					encoder.EncodeColon |
					encoder.EncodeRightPeriod |
					encoder.EncodeLeftSpace |
					encoder.EncodeRightSpace |
					encoder.EncodeBackSlash |
					encoder.EncodeRightSpace |
					encoder.EncodeLeftCrLfHtVt |
					encoder.EncodeRightCrLfHtVt |
					encoder.EncodeInvalidUtf8),
			},
		},
	})
}

// Options defines the configuration for this backend
type Options struct {
	ChunkSize      fs.SizeSuffix        `config:"chunk_size"`
	TmpDir         string               `config:"tmp_dir"`
	QueryPerMinute int                  `config:"query_per_minute"`
	Enc            encoder.MultiEncoder `config:"encoding"`
}

type chunkSizeOption struct {
	name  string
	value string
}

// Config config this backend
func Config(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
	switch config.State {
	case "":
		return oauthutil.ConfigOut("choose_chunk_size", &oauthutil.Options{
			OAuth2Config: oauthConfig,
		})
	case "choose_chunk_size":
		httpClient, ts, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
		if err != nil {
			return nil, err
		}
		token, err := ts.Token()
		if err != nil {
			return nil, err
		}
		params := url.Values{}
		params.Set("method", "uinfo")
		params.Set("access_token", token.AccessToken)
		var resp api.UserResponse
		_, err = rest.NewClient(httpClient).CallJSON(ctx, &rest.Opts{
			Method:     "GET",
			Path:       "/rest/2.0/xpan/nas",
			RootURL:    xPanServerRootURL,
			Parameters: params,
		}, nil, &resp)
		if err != nil {
			return nil, err
		}
		if resp.ErrorNumber != 0 {
			return nil, api.Err(resp.ErrorNumber)
		}
		fs.Debugf(nil, "user: %d, vip: %d", resp.UK, resp.VipType)

		chunkSizeOptions := []chunkSizeOption{{
			name: "Best for user", value: fs.SizeSuffix(4 * 1024 * 1024).String(),
		}}

		if resp.VipType > 0 {
			chunkSizeOptions = append(chunkSizeOptions, chunkSizeOption{
				name: "Best for VIP user", value: fs.SizeSuffix(16 * 1024 * 1024).String(),
			})
		}
		if resp.VipType > 1 {
			chunkSizeOptions = append(chunkSizeOptions, chunkSizeOption{
				name: "Best for Super VIP user", value: fs.SizeSuffix(16 * 1024 * 1024).String(),
			})
		}
		return fs.ConfigChoose("chunk_size", "chunk_size", "Chunk Size", len(chunkSizeOptions), func(i int) (string, string) {
			return chunkSizeOptions[i].name, chunkSizeOptions[i].value
		})
	case "chunk_size":
		m.Set("chunk_size", config.Result)
	default:
	}
	return nil, nil
}

// NewFs create this backend
func NewFs(ctx context.Context, name string, root string, config configmap.Mapper) (fs.Fs, error) {
	opts := new(Options)
	err := configstruct.Set(config, opts)
	if err != nil {
		return nil, err
	}
	httpClient, ts, err := oauthutil.NewClient(ctx, name, config, oauthConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to configure xpan: %w", err)
	}

	f := &Fs{
		name: name,
		ts:   ts,
		srv:  newRatelimiterClient(httpClient, opts.QueryPerMinute),
		pacer: fs.NewPacer(ctx, pacer.NewDefault(
			pacer.MinSleep(pacerMinSleep),
			pacer.MaxSleep(pacerMaxSleep),
			pacer.DecayConstant(pacerDecayConstant))),
		opts: *opts,
	}
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
		CaseInsensitive:         true,
	}).Fill(ctx, f)

	// test root
	rootItem, err := f.readFileMetaData(ctx, root)
	if err != nil && !errors.Is(err, fs.ErrorObjectNotFound) {
		return nil, err
	}

	// if root is a file return an error
	if err == nil && !rootItem.IsDir() {
		f.root = strings.Trim(path.Dir(root), "/")
		return f, fs.ErrorIsFile
	}

	// set fs root
	f.root = strings.Trim(root, "/")

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.ts.Token()
		return err
	})

	// ensure tmp_dir is created
	_ = os.Mkdir(f.opts.TmpDir, 0777)
	return f, nil
}
