// Package smugmug provides an interface to SmugMug albums.
package smugmug

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	apiOrigin         = "https://api.smugmug.com"
	uploadURL         = "https://upload.smugmug.com/"
	requestTokenURL   = "https://secure.smugmug.com/services/oauth/1.0a/getRequestToken"
	authorizeURL      = "https://secure.smugmug.com/services/oauth/1.0a/authorize"
	accessTokenURL    = "https://secure.smugmug.com/services/oauth/1.0a/getAccessToken"
	defaultAPIKey     = "2FHXLx2mJL8CKgKpSH2nNP995WSh3pDF"
	defaultAPISecret  = "gaIsi86AOxX6OTcrU4RCh6Ym5R5OIK0Qedf2g37dX4czUuLKfKTcMVvOtor3XpHt2Zo2Hqx7b1n0OAhqYsqyow9lVmSciufGZeKIoyCyD1I"
	listChunkSize     = 100
	minSleep          = 10 * time.Millisecond
	maxSleep          = 2 * time.Second
	cacheFilePrefix   = "rclone-smugmug-upload-"
	configAccessToken = "access_token"

	obscureIVSize            = 16
	smugMugAPISecretLength   = 64
	minAccessTokenSecretSize = 16
)

var (
	albumURIRe = regexp.MustCompile(`/api/v2/album/[A-Za-z0-9]+`)
	nickNameRe = regexp.MustCompile(`"nickName":"([^"]+)"`)
	nodeIDRe   = regexp.MustCompile(`sm-page-node-([A-Za-z0-9]+)`)
)

var retryErrorCodes = []int{
	429, // Too Many Requests
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
}

var errNodeNotFound = errors.New("SmugMug node not found")

var systemMetadataInfo = map[string]fs.MetadataHelp{
	"title": {
		Help:    "SmugMug image title",
		Type:    "string",
		Example: "Trip cover",
	},
	"caption": {
		Help:    "SmugMug image caption",
		Type:    "string",
		Example: "Taken from the trail",
	},
	"keywords": {
		Help:    "SmugMug image keywords",
		Type:    "string",
		Example: "travel,landscape",
	},
	"hidden": {
		Help:    "Whether the image is hidden in SmugMug",
		Type:    "bool",
		Example: "false",
	},
	"latitude": {
		Help:    "Image latitude in decimal degrees",
		Type:    "float",
		Example: "35.681236",
	},
	"longitude": {
		Help:    "Image longitude in decimal degrees",
		Type:    "float",
		Example: "139.767125",
	},
	"altitude": {
		Help:    "Image altitude in meters",
		Type:    "float",
		Example: "12.5",
	},
}

var commandHelp = []fs.CommandHelp{{
	Name:  "root",
	Short: "Show the authenticated SmugMug user and root node.",
	Long: `This command shows the authenticated SmugMug account and the root node
URI used for library mode.

Usage examples:

` + "```console" + `
rclone backend root smug:
` + "```",
}, {
	Name:  "list",
	Short: "List SmugMug folders and albums under a node or path.",
	Long: `This command lists SmugMug folder and album nodes.

By default it lists the configured root_node, or the authenticated user's root
node if root_node is not configured. Use -o node=/api/v2/node/abc123 to list a
specific node, or pass a path relative to the root node.

Usage examples:

` + "```console" + `
rclone backend list smug:
rclone backend list smug: Projects -o recursive=true
rclone backend list smug: -o node=/api/v2/node/abc123
` + "```",
	Opts: map[string]string{
		"node":      "Node URI or node ID to list.",
		"path":      "Path relative to the root node to list.",
		"recursive": "Recursively list folders.",
	},
}, {
	Name:  "list-folders",
	Short: "List SmugMug folders under a node or path.",
	Long: `This is like the list command, but only returns folder nodes.

Usage example:

` + "```console" + `
rclone backend list-folders smug: Projects -o recursive=true
` + "```",
	Opts: map[string]string{
		"node":      "Node URI or node ID to list.",
		"path":      "Path relative to the root node to list.",
		"recursive": "Recursively list folders.",
	},
}, {
	Name:  "list-albums",
	Short: "List SmugMug albums under a node or path.",
	Long: `This is like the list command, but only returns album nodes.

Usage example:

` + "```console" + `
rclone backend list-albums smug: Projects -o recursive=true
` + "```",
	Opts: map[string]string{
		"node":      "Node URI or node ID to list.",
		"path":      "Path relative to the root node to list.",
		"recursive": "Recursively list folders.",
	},
}, {
	Name:  "create-album",
	Short: "Create a SmugMug album under a folder node.",
	Long: `This command creates an album below a SmugMug folder node and returns the
new node URI, album URI, and web URL.

Pass the album name as the first argument or with -o name=. Pass the parent
folder with -o parent=/api/v2/node/abc123 or -o path=Projects. Privacy defaults to
Private if not supplied.

Usage examples:

` + "```console" + `
rclone backend create-album smug: "BlueMesa" -o path=Projects
rclone backend create-album smug: -o parent=/api/v2/node/abc123 -o name="New Album" -o privacy=Unlisted
` + "```",
	Opts: map[string]string{
		"parent":   "Parent folder node URI or node ID.",
		"path":     "Parent folder path relative to the root node.",
		"name":     "Album display name.",
		"url_name": "Album URL name.",
		"privacy":  "Album privacy: Private, Unlisted, or Public. Defaults to Private.",
	},
}, {
	Name:  "create-folder",
	Short: "Create a SmugMug folder under a folder node.",
	Long: `This command creates a folder below a SmugMug folder node and returns the
new node URI and web URL.

Pass the folder name as the first argument or with -o name=. Pass the parent
folder with -o parent=/api/v2/node/abc123 or -o path=Projects. Privacy defaults to
Private if not supplied.

Usage examples:

` + "```console" + `
rclone backend create-folder smug: "BlueMesa" -o path=Projects
rclone backend create-folder smug: -o parent=/api/v2/node/abc123 -o name="BlueMesa"
` + "```",
	Opts: map[string]string{
		"parent":   "Parent folder node URI or node ID.",
		"path":     "Parent folder path relative to the root node.",
		"name":     "Folder display name.",
		"url_name": "Folder URL name.",
		"privacy":  "Folder privacy: Private, Unlisted, or Public. Defaults to Private.",
	},
}, {
	Name:  "copy-image",
	Short: "Copy a SmugMug image to another album path.",
	Long: `This command copies one SmugMug image to another path. It can copy
between albums in library mode when the destination path is inside an existing
album.

SmugMug's exposed copy API does not accept a destination album, so this command
streams the image through rclone and uploads it to the destination.

Usage examples:

` + "```console" + `
rclone backend copy-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend copy-image smuglib: -o src=Projects/BlueMesa/photo.jpg -o dst=Projects/RiverLight/photo.jpg
` + "```",
	Opts: map[string]string{
		"src": "Source image path.",
		"dst": "Destination image path.",
	},
}, {
	Name:  "move-image",
	Short: "Move a SmugMug image to another album path.",
	Long: `This command moves one SmugMug image to another path. It uploads the
image to the destination first, then removes the source image only after the
upload succeeds.

SmugMug's exposed copy API does not accept a destination album, so this command
streams the image through rclone and uploads it to the destination.

Usage examples:

` + "```console" + `
rclone backend move-image smuglib: Projects/BlueMesa/photo.jpg Projects/RiverLight/photo.jpg
rclone backend move-image smuglib: -o src=Projects/BlueMesa/photo.jpg -o dst=Projects/RiverLight/photo.jpg
` + "```",
	Opts: map[string]string{
		"src": "Source image path.",
		"dst": "Destination image path.",
	},
}}

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "smugmug",
		Description: "SmugMug",
		NewFs:       NewFs,
		Config:      Config,
		MetadataInfo: &fs.MetadataInfo{
			System: systemMetadataInfo,
			Help:   "SmugMug image metadata is mapped to SmugMug image fields. Unsupported metadata keys are ignored when writing.",
		},
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: "album_uri",
			Help: "SmugMug album API URI, album key, or web URL to upload into.\n\nUse values like `/api/v2/album/AbCdEf`, `AbCdEf`, or `https://photos.example.com/2023/My-Album/i-AbCdEf/A`.\n\nLeave blank to use library mode.",
		}, {
			Name:    "root_node",
			Help:    "SmugMug root node API URI or node ID for library mode.\n\nBy default rclone presents the authenticated user's SmugMug folders and albums as a filesystem tree. Use `root` or `authuser` for the authenticated user's root node.",
			Default: "root",
		}, {
			Name:      "api_key",
			Help:      "SmugMug API key.\n\nLeave blank normally to use rclone's bundled development key.",
			Default:   defaultAPIKey,
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:       "api_secret",
			Help:       "SmugMug API key secret.\n\nLeave blank normally to use rclone's bundled development key.",
			Advanced:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name:      configAccessToken,
			Help:      "OAuth access token.\n\nThis is normally set by `rclone config`.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:       "access_token_secret",
			Help:       "OAuth access token secret.\n\nThis is normally set by `rclone config`.",
			Advanced:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name:     "md5_memory_limit",
			Help:     "Files bigger than this will be cached on disk when rclone must calculate upload MD5.",
			Default:  fs.SizeSuffix(32 * 1024 * 1024),
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Base |
				encoder.EncodeSlash |
				encoder.EncodeInvalidUtf8 |
				encoder.EncodeCtl |
				encoder.EncodeDel |
				encoder.EncodeBackSlash |
				encoder.EncodeHash |
				encoder.EncodePercent |
				encoder.EncodeQuestion),
		}},
	})
}

// Options defines the configuration for this backend.
type Options struct {
	AlbumURI          string               `config:"album_uri"`
	RootNode          string               `config:"root_node"`
	APIKey            string               `config:"api_key"`
	APISecret         string               `config:"api_secret"`
	AccessToken       string               `config:"access_token"`
	AccessTokenSecret string               `config:"access_token_secret"`
	MD5MemoryLimit    fs.SizeSuffix        `config:"md5_memory_limit"`
	Enc               encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a SmugMug album.
type Fs struct {
	name           string
	root           string
	opt            Options
	albumURI       string
	rootNodeURI    string
	library        bool
	features       *fs.Features
	client         *http.Client
	downloadClient *http.Client
	pacer          *fs.Pacer
}

// Object describes a SmugMug image.
type Object struct {
	fs            *Fs
	remote        string
	albumURI      string
	albumRemote   string
	size          int64
	modTime       time.Time
	contentType   string
	title         string
	caption       string
	keywords      string
	hidden        *bool
	latitude      *float64
	longitude     *float64
	altitude      *float64
	webURI        string
	imageURI      string
	albumImageURI string
	downloadURL   string
}

type apiResponse struct {
	Response struct {
		Uri        string       `json:"Uri"`
		Album      *album       `json:"Album"`
		AlbumImage []albumImage `json:"AlbumImage"`
		Image      []albumImage `json:"Image"`
		Node       *node        `json:"Node"`
		Pages      pages        `json:"Pages"`
	} `json:"Response"`
	Code    int    `json:"Code"`
	Message string `json:"Message"`
}

type apiLink struct {
	Uri string `json:"Uri"`
}

func (l *apiLink) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		l.Uri = s
		return nil
	}
	type link apiLink
	return json.Unmarshal(b, (*link)(l))
}

type apiStringList string

func (l *apiStringList) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		*l = apiStringList(s)
		return nil
	}
	var ss []string
	if err := json.Unmarshal(b, &ss); err == nil {
		*l = apiStringList(strings.Join(ss, ","))
		return nil
	}
	return fmt.Errorf("unexpected string list value %s", string(b))
}

type apiFloat struct {
	value float64
	valid bool
}

func (f *apiFloat) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var n float64
	if err := json.Unmarshal(b, &n); err == nil {
		f.value = n
		f.valid = true
		return nil
	}
	var s string
	if err := json.Unmarshal(b, &s); err == nil {
		if strings.TrimSpace(s) == "" {
			return nil
		}
		value, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return err
		}
		f.value = value
		f.valid = true
		return nil
	}
	return fmt.Errorf("unexpected float value %s", string(b))
}

func (f apiFloat) ptr() *float64 {
	if !f.valid {
		return nil
	}
	value := f.value
	return &value
}

type album struct {
	Uri     string             `json:"Uri"`
	Name    string             `json:"Name"`
	UrlName string             `json:"UrlName"`
	WebUri  string             `json:"WebUri"`
	Uris    map[string]apiLink `json:"Uris"`
}

type node struct {
	Name         string             `json:"Name"`
	Uri          string             `json:"Uri"`
	NodeID       string             `json:"NodeID"`
	Type         string             `json:"Type"`
	UrlName      string             `json:"UrlName"`
	UrlPath      string             `json:"UrlPath"`
	WebUri       string             `json:"WebUri"`
	DateAdded    string             `json:"DateAdded"`
	DateModified string             `json:"DateModified"`
	Uris         map[string]apiLink `json:"Uris"`
}

type pages struct {
	NextPage string `json:"NextPage"`
}

type authUserResponse struct {
	Response struct {
		User user `json:"User"`
	} `json:"Response"`
}

type user struct {
	Name     string             `json:"Name"`
	NickName string             `json:"NickName"`
	Uri      string             `json:"Uri"`
	WebUri   string             `json:"WebUri"`
	Uris     map[string]apiLink `json:"Uris"`
}

type nodeListResponse struct {
	Response struct {
		Node  []node `json:"Node"`
		Pages pages  `json:"Pages"`
	} `json:"Response"`
}

type libraryLocation struct {
	node        node
	albumURI    string
	albumPrefix string
	nodePath    string
}

type commandRootInfo struct {
	Name        string `json:"name"`
	NickName    string `json:"nick_name"`
	UserURI     string `json:"user_uri"`
	WebURI      string `json:"web_uri"`
	RootNodeURI string `json:"root_node_uri"`
}

type commandNodeInfo struct {
	Type     string `json:"type"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	NodeURI  string `json:"node_uri"`
	AlbumURI string `json:"album_uri,omitempty"`
	WebURI   string `json:"web_uri"`
}

type commandImageTransferInfo struct {
	Mode                string `json:"mode"`
	Source              string `json:"source"`
	Destination         string `json:"destination"`
	SourceAlbumURI      string `json:"source_album_uri,omitempty"`
	DestinationAlbumURI string `json:"destination_album_uri,omitempty"`
	AlbumImageURI       string `json:"album_image_uri,omitempty"`
	ImageURI            string `json:"image_uri,omitempty"`
	ServerSide          bool   `json:"server_side"`
}

type albumImage struct {
	Uri          string             `json:"Uri"`
	FileName     string             `json:"FileName"`
	Title        string             `json:"Title"`
	Caption      string             `json:"Caption"`
	Keywords     apiStringList      `json:"Keywords"`
	Hidden       *bool              `json:"Hidden"`
	Latitude     apiFloat           `json:"Latitude"`
	Longitude    apiFloat           `json:"Longitude"`
	Altitude     apiFloat           `json:"Altitude"`
	ArchivedUri  string             `json:"ArchivedUri"`
	OriginalSize int64              `json:"OriginalSize"`
	Size         int64              `json:"Size"`
	Date         string             `json:"Date"`
	LastUpdated  string             `json:"LastUpdated"`
	Format       string             `json:"Format"`
	MimeType     string             `json:"MimeType"`
	WebUri       string             `json:"WebUri"`
	Uris         map[string]apiLink `json:"Uris"`
}

type uploadResponse struct {
	Stat    string `json:"stat"`
	Message string `json:"message"`
	Image   struct {
		ImageUri      string `json:"ImageUri"`
		AlbumImageUri string `json:"AlbumImageUri"`
		URL           string `json:"URL"`
	} `json:"Image"`
}

type oauthCredentials struct {
	consumerKey    string
	consumerSecret string
	token          string
	tokenSecret    string
}

type oauth1Transport struct {
	base http.RoundTripper
	cred oauthCredentials
}

// Config performs the SmugMug OAuth 1.0a authorization flow.
func Config(ctx context.Context, name string, m configmap.Mapper, in fs.ConfigIn) (*fs.ConfigOut, error) {
	opt, err := getOptions(m)
	if err != nil {
		return nil, err
	}
	if opt.AccessToken != "" && opt.AccessTokenSecret != "" {
		return nil, nil
	}

	switch {
	case in.State == "":
		token, secret, err := requestToken(ctx, opt)
		if err != nil {
			return nil, fmt.Errorf("failed to get SmugMug request token: %w", err)
		}
		authURL, err := makeAuthorizeURL(token)
		if err != nil {
			return nil, err
		}
		state := fs.StatePush("", "oauth-verifier", token, secret)
		return fs.ConfigInput(
			state,
			"config_verification_code",
			fmt.Sprintf("Go to this URL, authorize rclone, then paste the six-digit verification code here.\n\n%s", authURL),
		)
	case strings.HasPrefix(in.State, "oauth-verifier"):
		state, _ := fs.StatePop(in.State)
		state, requestToken := fs.StatePop(state)
		_, requestSecret := fs.StatePop(state)
		accessToken, accessSecret, err := accessToken(ctx, opt, requestToken, requestSecret, strings.TrimSpace(in.Result))
		if err != nil {
			return fs.ConfigError("", fmt.Sprintf("Failed to complete SmugMug OAuth: %v", err))
		}
		m.Set(configAccessToken, accessToken)
		m.Set("access_token_secret", obscure.MustObscure(accessSecret))
		return nil, nil
	}
	return nil, fmt.Errorf("unknown state %q", in.State)
}

func getOptions(m configmap.Mapper) (*Options, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}
	if opt.APIKey == "" {
		opt.APIKey = defaultAPIKey
	}
	var err error
	if opt.APISecret == "" {
		opt.APISecret = obscure.MustReveal(defaultAPISecret)
	} else {
		opt.APISecret, err = revealObscured("api_secret", opt.APISecret, smugMugAPISecretLength)
		if err != nil {
			return nil, err
		}
	}
	if opt.AccessTokenSecret != "" {
		opt.AccessTokenSecret, err = revealObscured("access_token_secret", opt.AccessTokenSecret, minAccessTokenSecretSize)
		if err != nil {
			return nil, err
		}
	}
	if opt.AlbumURI == "" && opt.RootNode == "" {
		opt.RootNode = "root"
	}
	return opt, nil
}

func revealObscured(name, in string, minPlainSize int) (string, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(in)
	if err != nil {
		return "", fmt.Errorf("%s must be stored obscured; use `rclone config` or `rclone obscure`: %w", name, err)
	}
	if len(decoded) < obscureIVSize+minPlainSize {
		return "", fmt.Errorf("%s must be stored obscured; use `rclone config` or `rclone obscure`", name)
	}
	out, err := obscure.Reveal(in)
	if err != nil {
		return "", fmt.Errorf("failed to reveal %s: %w", name, err)
	}
	return out, nil
}

// NewFs constructs an Fs from the path.
func NewFs(ctx context.Context, name string, root string, m configmap.Mapper) (fs.Fs, error) {
	opt, err := getOptions(m)
	if err != nil {
		return nil, err
	}
	if opt.AccessToken == "" || opt.AccessTokenSecret == "" {
		return nil, errors.New("missing SmugMug OAuth token; run `rclone config` for this remote")
	}
	root = strings.Trim(path.Clean(root), "/")
	if root == "." || root == "/" {
		root = ""
	}

	baseClient := fshttp.NewClient(ctx)
	client := *baseClient
	client.Transport = &oauth1Transport{
		base: baseClient.Transport,
		cred: oauthCredentials{
			consumerKey:    opt.APIKey,
			consumerSecret: opt.APISecret,
			token:          opt.AccessToken,
			tokenSecret:    opt.AccessTokenSecret,
		},
	}

	f := &Fs{
		name:           name,
		root:           root,
		opt:            *opt,
		client:         &client,
		downloadClient: baseClient,
		pacer:          fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep))),
	}
	if opt.AlbumURI != "" {
		f.albumURI, err = f.resolveAlbumURI(ctx, opt.AlbumURI)
		if err != nil {
			return nil, err
		}
	} else {
		f.rootNodeURI, err = f.resolveRootNodeURI(ctx, opt.RootNode)
		if err != nil {
			return nil, err
		}
		f.library = true
	}
	features := &fs.Features{
		DuplicateFiles: true,
		ReadMimeType:   true,
		ReadMetadata:   true,
		WriteMetadata:  true,
	}
	if f.library {
		features.CanHaveEmptyDirectories = true
	}
	f.features = features.Fill(ctx, f)
	if root != "" {
		remote := path.Base(root)
		f.root = cleanRemote(path.Dir(root))
		_, err := f.NewObject(ctx, remote)
		if err != nil {
			if err == fs.ErrorObjectNotFound || errors.Is(err, fs.ErrorNotAFile) {
				f.root = root
				return f, nil
			}
			return nil, err
		}
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// Name of the remote.
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote.
func (f *Fs) Root() string {
	return f.root
}

// String returns a description of the Fs.
func (f *Fs) String() string {
	if f.library {
		return fmt.Sprintf("SmugMug library %s", f.rootNodeURI)
	}
	return fmt.Sprintf("SmugMug album %s", f.albumURI)
}

// Precision of the ModTimes in this Fs.
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash types of the filesystem.
func (f *Fs) Hashes() hash.Set {
	return hash.NewHashSet()
}

// Features returns the optional features of this Fs.
func (f *Fs) Features() *fs.Features {
	return f.features
}

// List the objects and directories in dir.
func (f *Fs) List(ctx context.Context, dir string) (fs.DirEntries, error) {
	if f.library {
		return f.listLibrary(ctx, dir)
	}

	images, err := f.listImages(ctx)
	if err != nil {
		return nil, err
	}

	prefix := cleanRemote(path.Join(f.root, dir))
	if prefix != "" {
		prefix += "/"
	}

	var entries fs.DirEntries
	seenDirs := map[string]struct{}{}
	for _, image := range images {
		remote := f.remoteFromImage(image)
		if prefix != "" {
			if !strings.HasPrefix(remote, prefix) {
				continue
			}
			remote = strings.TrimPrefix(remote, prefix)
		}
		if remote == "" {
			continue
		}
		if i := strings.Index(remote, "/"); i >= 0 {
			dirRemote := cleanRemote(path.Join(dir, remote[:i]))
			if _, ok := seenDirs[dirRemote]; !ok {
				entries = append(entries, fs.NewDir(dirRemote, parseSmugMugTime(image.LastUpdated)))
				seenDirs[dirRemote] = struct{}{}
			}
			continue
		}
		objRemote := cleanRemote(path.Join(dir, remote))
		entries = append(entries, f.newObjectFromImage(objRemote, image))
	}
	if dir != "" && len(entries) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	return entries, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	if f.library {
		return f.newLibraryObject(ctx, remote)
	}

	images, err := f.listImages(ctx)
	if err != nil {
		return nil, err
	}
	want := cleanRemote(path.Join(f.root, remote))
	for _, image := range images {
		if f.remoteFromImage(image) == want {
			return f.newObjectFromImage(remote, image), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

// Put uploads a new object.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := &Object{
		fs:      f,
		remote:  src.Remote(),
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}
	err := o.Update(ctx, in, src, options...)
	if err != nil {
		return nil, err
	}
	return o, nil
}

// Mkdir creates a SmugMug folder in library mode, or a virtual directory in album mode.
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	if !f.library {
		return nil
	}
	dir = cleanRemote(dir)
	fullDir := cleanRemote(path.Join(f.root, dir))
	if fullDir == "" {
		return nil
	}
	loc, err := f.resolveLibraryPath(ctx, fullDir)
	if err == nil {
		return mkdirExistingLibraryPath(dir, fullDir, loc)
	}
	if !errors.Is(err, errNodeNotFound) {
		return err
	}

	parentPath, leaf := path.Split(fullDir)
	leaf = strings.Trim(leaf, "/")
	if leaf == "" {
		return nil
	}
	parentPath = cleanRemote(parentPath)
	parent, err := f.resolveLibraryPath(ctx, parentPath)
	if errors.Is(err, errNodeNotFound) {
		return fs.ErrorDirNotFound
	}
	if err != nil {
		return err
	}
	if parent.albumPrefix != "" || parent.node.Type != "Folder" {
		return fmt.Errorf("SmugMug parent path %q is not a folder", parentPath)
	}
	_, err = f.createNode(ctx, parent.node.Uri, parent.nodePath, "Folder", f.opt.Enc.FromStandardName(leaf), "", "")
	return err
}

func mkdirExistingLibraryPath(dir, fullDir string, loc *libraryLocation) error {
	if loc.albumPrefix != "" {
		if dir == "" {
			return nil
		}
		return fmt.Errorf("SmugMug album path %q is virtual and can't store empty directories", fullDir)
	}
	if loc.node.Type == "Folder" || loc.node.Type == "Album" {
		return nil
	}
	return fmt.Errorf("SmugMug path %q is a %s, not a folder", fullDir, loc.node.Type)
}

// Rmdir removes a virtual directory if it is empty.
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	dir = cleanRemote(dir)
	entries, err := f.List(ctx, dir)
	if err == fs.ErrorDirNotFound {
		return nil
	}
	if err != nil {
		return err
	}
	if len(entries) != 0 {
		return fs.ErrorDirectoryNotEmpty
	}
	if f.library {
		fullDir := cleanRemote(path.Join(f.root, dir))
		if fullDir == "" {
			return nil
		}
		loc, err := f.resolveLibraryPath(ctx, fullDir)
		if errors.Is(err, errNodeNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		if loc.albumPrefix != "" {
			return nil
		}
		if loc.node.Type != "Folder" {
			return fmt.Errorf("rmdir only removes SmugMug folders; %q is a %s", fullDir, loc.node.Type)
		}
		return f.doJSON(ctx, http.MethodDelete, loc.node.Uri, nil, nil)
	}
	return nil
}

// PublicLink returns the SmugMug web URL for an object, folder, or album.
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (string, error) {
	if unlink || expire != fs.DurationOff {
		return "", fs.ErrorNotImplemented
	}
	remote = cleanRemote(remote)
	if remote != "" {
		obj, err := f.NewObject(ctx, remote)
		if err == nil {
			smugObj, ok := obj.(*Object)
			if !ok {
				return "", fs.ErrorNotAFile
			}
			return smugObj.publicLink(ctx)
		}
		if err != fs.ErrorObjectNotFound {
			return "", err
		}
	}

	if f.library {
		fullRemote := cleanRemote(path.Join(f.root, remote))
		loc, err := f.resolveLibraryPath(ctx, fullRemote)
		if errors.Is(err, errNodeNotFound) {
			return "", fs.ErrorDirNotFound
		}
		if err != nil {
			return "", err
		}
		if loc.albumPrefix != "" {
			return "", fs.ErrorCantShareDirectories
		}
		if loc.node.WebUri == "" {
			return "", errors.New("SmugMug node response did not include a web URL")
		}
		return loc.node.WebUri, nil
	}

	if remote == "" {
		album, err := f.getAlbum(ctx, f.albumURI)
		if err != nil {
			return "", err
		}
		if album.WebUri == "" {
			return "", errors.New("SmugMug album response did not include a web URL")
		}
		return album.WebUri, nil
	}
	return "", fs.ErrorCantShareDirectories
}

func (f *Fs) listLibrary(ctx context.Context, dir string) (fs.DirEntries, error) {
	fullDir := cleanRemote(path.Join(f.root, dir))
	loc, err := f.resolveLibraryPath(ctx, fullDir)
	if errors.Is(err, errNodeNotFound) {
		return nil, fs.ErrorDirNotFound
	}
	if err != nil {
		return nil, err
	}
	if loc.node.Type == "Album" {
		return f.listAlbumEntries(ctx, dir, loc.albumURI, loc.albumPrefix)
	}

	children, err := f.listChildNodes(ctx, loc.node.Uri)
	if err != nil {
		return nil, err
	}
	entries := make(fs.DirEntries, 0, len(children))
	for _, child := range children {
		if child.Type != "Folder" && child.Type != "Album" {
			continue
		}
		remote := cleanRemote(path.Join(dir, f.nodeName(child)))
		entries = append(entries, fs.NewDir(remote, parseSmugMugTime(firstNonEmpty(child.DateModified, child.DateAdded))))
	}
	return entries, nil
}

func (f *Fs) listAlbumEntries(ctx context.Context, dir, albumURI, imagePrefix string) (fs.DirEntries, error) {
	images, err := f.listAlbumImages(ctx, albumURI)
	if err != nil {
		return nil, err
	}

	imagePrefix = cleanRemote(imagePrefix)
	prefix := imagePrefix
	if prefix != "" {
		prefix += "/"
	}

	var entries fs.DirEntries
	seenDirs := map[string]struct{}{}
	for _, image := range images {
		albumRemote := f.remoteFromImage(image)
		remaining := albumRemote
		if prefix != "" {
			if !strings.HasPrefix(albumRemote, prefix) {
				continue
			}
			remaining = strings.TrimPrefix(albumRemote, prefix)
		}
		if remaining == "" {
			continue
		}
		if i := strings.Index(remaining, "/"); i >= 0 {
			dirRemote := cleanRemote(path.Join(dir, remaining[:i]))
			if _, ok := seenDirs[dirRemote]; !ok {
				entries = append(entries, fs.NewDir(dirRemote, parseSmugMugTime(image.LastUpdated)))
				seenDirs[dirRemote] = struct{}{}
			}
			continue
		}
		objRemote := cleanRemote(path.Join(dir, remaining))
		entries = append(entries, f.newObjectFromImageInAlbum(objRemote, image, albumURI, albumRemote))
	}
	if imagePrefix != "" && len(entries) == 0 {
		return nil, fs.ErrorDirNotFound
	}
	return entries, nil
}

func (f *Fs) newLibraryObject(ctx context.Context, remote string) (fs.Object, error) {
	fullRemote := cleanRemote(path.Join(f.root, remote))
	loc, albumRemote, err := f.resolveLibraryObjectPath(ctx, fullRemote)
	if errors.Is(err, errNodeNotFound) {
		return nil, fs.ErrorObjectNotFound
	}
	if err != nil {
		return nil, err
	}
	images, err := f.listAlbumImages(ctx, loc.albumURI)
	if err != nil {
		return nil, err
	}
	for _, image := range images {
		if f.remoteFromImage(image) == albumRemote {
			return f.newObjectFromImageInAlbum(remote, image, loc.albumURI, albumRemote), nil
		}
	}
	return nil, fs.ErrorObjectNotFound
}

func (f *Fs) uploadTarget(ctx context.Context, remote string) (albumURI, albumRemote string, err error) {
	if !f.library {
		albumRemote := cleanRemote(path.Join(f.root, remote))
		if albumRemote == "" {
			return "", "", errors.New("can't upload object with empty name to SmugMug")
		}
		return f.albumURI, albumRemote, nil
	}
	fullRemote := cleanRemote(path.Join(f.root, remote))
	loc, albumRemote, err := f.resolveLibraryObjectPath(ctx, fullRemote)
	if errors.Is(err, errNodeNotFound) {
		return "", "", fmt.Errorf("SmugMug path %q is not inside an existing album; create or choose an album first", fullRemote)
	}
	if err != nil {
		return "", "", err
	}
	return loc.albumURI, albumRemote, nil
}

func (f *Fs) resolveLibraryObjectPath(ctx context.Context, remote string) (*libraryLocation, string, error) {
	loc, err := f.resolveLibraryPath(ctx, remote)
	if err != nil {
		return nil, "", err
	}
	if loc.node.Type != "Album" || loc.albumPrefix == "" {
		return nil, "", errNodeNotFound
	}
	return loc, loc.albumPrefix, nil
}

func (f *Fs) resolveLibraryPath(ctx context.Context, remote string) (*libraryLocation, error) {
	return f.resolveLibraryPathFrom(ctx, f.rootNodeURI, remote)
}

func (f *Fs) resolveLibraryPathFrom(ctx context.Context, rootNodeURI, remote string) (*libraryLocation, error) {
	root, err := f.getNode(ctx, rootNodeURI)
	if err != nil {
		return nil, err
	}
	remote = cleanRemote(remote)
	if remote == "" {
		return &libraryLocation{
			node:     root,
			albumURI: f.albumURIFromNode(root),
		}, nil
	}

	current := root
	var nodePath []string
	parts := strings.Split(remote, "/")
	for i, part := range parts {
		if current.Type == "Album" {
			return &libraryLocation{
				node:        current,
				albumURI:    f.albumURIFromNode(current),
				albumPrefix: strings.Join(parts[i:], "/"),
				nodePath:    strings.Join(nodePath, "/"),
			}, nil
		}
		children, err := f.listChildNodes(ctx, current.Uri)
		if err != nil {
			return nil, err
		}
		child, ok := f.findChildNode(children, part)
		if !ok {
			return nil, errNodeNotFound
		}
		current = child
		nodePath = append(nodePath, f.nodeName(child))
		if current.Type == "Album" {
			return &libraryLocation{
				node:        current,
				albumURI:    f.albumURIFromNode(current),
				albumPrefix: strings.Join(parts[i+1:], "/"),
				nodePath:    strings.Join(nodePath, "/"),
			}, nil
		}
	}
	return &libraryLocation{
		node:     current,
		albumURI: f.albumURIFromNode(current),
		nodePath: strings.Join(nodePath, "/"),
	}, nil
}

func (f *Fs) findChildNode(nodes []node, name string) (node, bool) {
	for _, item := range nodes {
		if f.nodeName(item) == name || item.UrlName == name || path.Base(item.UrlPath) == name {
			return item, true
		}
	}
	return node{}, false
}

func (f *Fs) nodeName(item node) string {
	name := item.Name
	if name == "" {
		name = item.UrlName
	}
	if name == "" {
		name = path.Base(strings.Trim(item.UrlPath, "/"))
	}
	if name == "" {
		name = item.NodeID
	}
	return cleanRemote(f.opt.Enc.ToStandardName(name))
}

func (f *Fs) albumURIFromNode(item node) string {
	if item.Uris == nil {
		return ""
	}
	return item.Uris["Album"].Uri
}

func (f *Fs) listImages(ctx context.Context) ([]albumImage, error) {
	return f.listAlbumImages(ctx, f.albumURI)
}

func (f *Fs) listAlbumImages(ctx context.Context, albumURI string) ([]albumImage, error) {
	uri := fmt.Sprintf("%s!images?count=%d&_verbosity=1", albumURI, listChunkSize)
	var out []albumImage
	for uri != "" {
		var result apiResponse
		if err := f.doJSON(ctx, http.MethodGet, uri, nil, &result); err != nil {
			return nil, err
		}
		out = append(out, result.Response.AlbumImage...)
		out = append(out, result.Response.Image...)
		uri = result.Response.Pages.NextPage
	}
	return out, nil
}

func (f *Fs) getAuthUser(ctx context.Context) (user, error) {
	var result authUserResponse
	if err := f.doJSON(ctx, http.MethodGet, "/api/v2!authuser?_verbosity=1", nil, &result); err != nil {
		return user{}, err
	}
	return result.Response.User, nil
}

func (f *Fs) getAuthRootNodeURI(ctx context.Context) (string, error) {
	user, err := f.getAuthUser(ctx)
	if err != nil {
		return "", err
	}
	root := user.Uris["Node"].Uri
	if root == "" {
		return "", errors.New("SmugMug auth user response did not include a root node")
	}
	return root, nil
}

func (f *Fs) getNode(ctx context.Context, nodeURI string) (node, error) {
	nodeURI, err := normalizeNodeURI(nodeURI)
	if err != nil {
		return node{}, err
	}
	var result apiResponse
	if err := f.doJSON(ctx, http.MethodGet, addQuery(nodeURI, "_verbosity", "1"), nil, &result); err != nil {
		return node{}, err
	}
	if result.Response.Node == nil {
		return node{}, errNodeNotFound
	}
	return *result.Response.Node, nil
}

func (f *Fs) getAlbum(ctx context.Context, albumURI string) (album, error) {
	albumURI, err := normalizeAlbumURI(albumURI)
	if err != nil {
		return album{}, err
	}
	var result apiResponse
	if err := f.doJSON(ctx, http.MethodGet, addQuery(albumURI, "_verbosity", "1"), nil, &result); err != nil {
		return album{}, err
	}
	if result.Response.Album == nil {
		return album{}, fs.ErrorDirNotFound
	}
	return *result.Response.Album, nil
}

func (f *Fs) listChildNodes(ctx context.Context, nodeURI string) ([]node, error) {
	nodeURI, err := normalizeNodeURI(nodeURI)
	if err != nil {
		return nil, err
	}
	uri := fmt.Sprintf("%s!children?count=%d&_verbosity=1", nodeURI, listChunkSize)
	var out []node
	for uri != "" {
		var result nodeListResponse
		if err := f.doJSON(ctx, http.MethodGet, uri, nil, &result); err != nil {
			return nil, err
		}
		out = append(out, result.Response.Node...)
		uri = result.Response.Pages.NextPage
	}
	return out, nil
}

func (f *Fs) resolveRootNodeURI(ctx context.Context, in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", nil
	}
	switch strings.ToLower(in) {
	case "root", "authuser", "auth-user":
		return f.getAuthRootNodeURI(ctx)
	default:
		return normalizeNodeURI(in)
	}
}

func (f *Fs) commandStartNode(ctx context.Context, arg []string, opt map[string]string) (nodeURI, startPath string, err error) {
	if nodeOpt := strings.TrimSpace(opt["node"]); nodeOpt != "" {
		nodeURI, err := normalizeNodeURI(nodeOpt)
		return nodeURI, "", err
	}
	startPath = strings.TrimSpace(opt["path"])
	if startPath == "" && len(arg) > 0 {
		startPath = arg[0]
	}
	if startPath == "" {
		startPath = f.root
	}
	rootNodeURI := f.rootNodeURI
	if rootNodeURI == "" {
		rootNodeURI, err = f.getAuthRootNodeURI(ctx)
		if err != nil {
			return "", "", err
		}
	}
	if startPath == "" {
		return rootNodeURI, "", nil
	}
	loc, err := f.resolveNodePathFrom(ctx, rootNodeURI, startPath)
	if err != nil {
		return "", "", err
	}
	return loc.node.Uri, loc.nodePath, nil
}

func (f *Fs) resolveNodePathFrom(ctx context.Context, rootNodeURI, remote string) (*libraryLocation, error) {
	return f.resolveLibraryPathFrom(ctx, rootNodeURI, remote)
}

func (f *Fs) listCommandNodes(ctx context.Context, startNodeURI, startPath, kind string, recursive bool) ([]commandNodeInfo, error) {
	children, err := f.listChildNodes(ctx, startNodeURI)
	if err != nil {
		return nil, err
	}
	var out []commandNodeInfo
	for _, child := range children {
		if child.Type != "Folder" && child.Type != "Album" {
			continue
		}
		childPath := cleanRemote(path.Join(startPath, f.nodeName(child)))
		if kind == "" || child.Type == kind {
			out = append(out, f.commandNodeInfo(child, childPath))
		}
		if recursive && child.Type == "Folder" {
			sub, err := f.listCommandNodes(ctx, child.Uri, childPath, kind, true)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		}
	}
	return out, nil
}

func (f *Fs) commandNodeInfo(item node, itemPath string) commandNodeInfo {
	return commandNodeInfo{
		Type:     item.Type,
		Name:     f.nodeName(item),
		Path:     itemPath,
		NodeURI:  item.Uri,
		AlbumURI: f.albumURIFromNode(item),
		WebURI:   item.WebUri,
	}
}

func (f *Fs) commandNodeInfoInParent(item node, parentPath string) commandNodeInfo {
	return f.commandNodeInfo(item, cleanRemote(path.Join(parentPath, f.nodeName(item))))
}

func (f *Fs) createNode(ctx context.Context, parentURI, parentPath, kind, name, urlName, privacy string) (commandNodeInfo, error) {
	parentURI, err := normalizeNodeURI(parentURI)
	if err != nil {
		return commandNodeInfo{}, err
	}
	if name == "" {
		return commandNodeInfo{}, errors.New("name is required")
	}
	if privacy == "" {
		privacy = "Private"
	}
	in := map[string]string{
		"Type":    kind,
		"Name":    name,
		"Privacy": privacy,
	}
	if urlName != "" {
		in["UrlName"] = urlName
	}
	var result apiResponse
	if err := f.doJSON(ctx, http.MethodPost, parentURI+"!children", in, &result); err != nil {
		return commandNodeInfo{}, err
	}
	if result.Response.Node == nil {
		return commandNodeInfo{}, errors.New("SmugMug create response did not include a node")
	}
	item := *result.Response.Node
	if kind == "Album" && f.albumURIFromNode(item) == "" && item.NodeID != "" {
		if albumURI, err := f.resolveNodeAlbumURI(ctx, item.NodeID); err == nil {
			if item.Uris == nil {
				item.Uris = map[string]apiLink{}
			}
			item.Uris["Album"] = apiLink{Uri: albumURI}
		}
	}
	return f.commandNodeInfoInParent(item, parentPath), nil
}

func (f *Fs) commandCreateParent(ctx context.Context, opt map[string]string) (parentURI, parentPath string, err error) {
	if parent := strings.TrimSpace(opt["parent"]); parent != "" {
		parentURI, err = normalizeNodeURI(parent)
		return parentURI, "", err
	}
	if nodeOpt := strings.TrimSpace(opt["node"]); nodeOpt != "" {
		parentURI, err = normalizeNodeURI(nodeOpt)
		return parentURI, "", err
	}
	pathOpt := strings.TrimSpace(opt["path"])
	rootNodeURI := f.rootNodeURI
	if rootNodeURI == "" {
		rootNodeURI, err = f.getAuthRootNodeURI(ctx)
		if err != nil {
			return "", "", err
		}
	}
	if pathOpt == "" {
		pathOpt = f.root
	}
	if pathOpt == "" {
		return rootNodeURI, "", nil
	}
	loc, err := f.resolveNodePathFrom(ctx, rootNodeURI, pathOpt)
	if err != nil {
		return "", "", err
	}
	if loc.node.Type != "Folder" {
		return "", "", fmt.Errorf("SmugMug path %q is a %s, not a folder", pathOpt, loc.node.Type)
	}
	return loc.node.Uri, loc.nodePath, nil
}

func commandImageTransferArgs(arg []string, opt map[string]string) (src, dst string, err error) {
	src = strings.TrimSpace(opt["src"])
	dst = strings.TrimSpace(opt["dst"])
	positional := arg
	if src == "" && len(positional) > 0 {
		src = strings.TrimSpace(positional[0])
		positional = positional[1:]
	}
	if dst == "" && len(positional) > 0 {
		dst = strings.TrimSpace(positional[0])
		positional = positional[1:]
	}
	if len(positional) > 0 {
		return "", "", errors.New("source and destination are required; pass exactly two arguments or -o src= and -o dst=")
	}
	if src == "" || dst == "" {
		return "", "", errors.New("source and destination are required")
	}
	return src, dst, nil
}

func (f *Fs) copyOrMoveImageCommand(ctx context.Context, srcRemote, dstRemote string, move bool) (info commandImageTransferInfo, err error) {
	srcRemote = cleanRemote(srcRemote)
	dstRemote = cleanRemote(dstRemote)
	if srcRemote == "" || dstRemote == "" {
		return commandImageTransferInfo{}, errors.New("source and destination are required")
	}
	if srcRemote == dstRemote {
		return commandImageTransferInfo{}, errors.New("source and destination must be different")
	}

	srcObject, err := f.NewObject(ctx, srcRemote)
	if err != nil {
		return commandImageTransferInfo{}, err
	}
	src, ok := srcObject.(*Object)
	if !ok {
		return commandImageTransferInfo{}, fmt.Errorf("source %q is not a SmugMug object", srcRemote)
	}

	dstAlbumURI, _, err := f.uploadTarget(ctx, dstRemote)
	if err != nil {
		return commandImageTransferInfo{}, err
	}

	dstInfo := object.NewStaticObjectInfo(dstRemote, src.ModTime(ctx), src.Size(), true, nil, f).
		WithMimeType(src.MimeType(ctx))
	var options []fs.OpenOption
	if metadata := src.smugMugMetadata(); len(metadata) != 0 {
		options = append(options, fs.MetadataOption(metadata))
	}

	existing, err := f.NewObject(ctx, dstRemote)
	var dst *Object
	switch {
	case err == nil:
		var ok bool
		dst, ok = existing.(*Object)
		if !ok {
			return commandImageTransferInfo{}, fmt.Errorf("destination %q is not a SmugMug object", dstRemote)
		}
	case err == fs.ErrorObjectNotFound:
	default:
		return commandImageTransferInfo{}, err
	}

	in, err := src.Open(ctx)
	if err != nil {
		return commandImageTransferInfo{}, err
	}
	defer fs.CheckClose(in, &err)

	if dst != nil {
		err = dst.Update(ctx, in, dstInfo, options...)
	} else {
		var obj fs.Object
		obj, err = f.Put(ctx, in, dstInfo, options...)
		if err == nil {
			var ok bool
			dst, ok = obj.(*Object)
			if !ok {
				err = fmt.Errorf("destination %q is not a SmugMug object", dstRemote)
			}
		}
	}
	if err != nil {
		return commandImageTransferInfo{}, err
	}

	if move {
		if err = src.Remove(ctx); err != nil {
			return commandImageTransferInfo{}, err
		}
	}

	mode := "copy"
	if move {
		mode = "move"
	}
	if dst != nil && dst.albumURI != "" {
		dstAlbumURI = dst.albumURI
	}
	metadata := src.smugMugMetadata()
	if len(metadata) != 0 {
		dst.setMetadata(metadata)
	}
	return commandImageTransferInfo{
		Mode:                mode,
		Source:              srcRemote,
		Destination:         dstRemote,
		SourceAlbumURI:      src.albumURI,
		DestinationAlbumURI: dstAlbumURI,
		AlbumImageURI:       dst.albumImageURI,
		ImageURI:            dst.imageURI,
		ServerSide:          false,
	}, nil
}

// UserInfo returns info about the connected user.
func (f *Fs) UserInfo(ctx context.Context) (map[string]string, error) {
	user, err := f.getAuthUser(ctx)
	if err != nil {
		return nil, err
	}
	return map[string]string{
		"name":          user.Name,
		"nick_name":     user.NickName,
		"user_uri":      user.Uri,
		"web_uri":       user.WebUri,
		"root_node_uri": user.Uris["Node"].Uri,
	}, nil
}

// Command runs a SmugMug backend command.
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (any, error) {
	switch name {
	case "root":
		user, err := f.getAuthUser(ctx)
		if err != nil {
			return nil, err
		}
		return commandRootInfo{
			Name:        user.Name,
			NickName:    user.NickName,
			UserURI:     user.Uri,
			WebURI:      user.WebUri,
			RootNodeURI: user.Uris["Node"].Uri,
		}, nil
	case "list", "list-folders", "list-albums":
		startNodeURI, startPath, err := f.commandStartNode(ctx, arg, opt)
		if err != nil {
			return nil, err
		}
		recursive, err := parseBoolOption(opt, "recursive")
		if err != nil {
			return nil, err
		}
		kind := ""
		if name == "list-folders" {
			kind = "Folder"
		} else if name == "list-albums" {
			kind = "Album"
		}
		return f.listCommandNodes(ctx, startNodeURI, startPath, kind, recursive)
	case "create-album", "create-folder":
		nodeName := strings.TrimSpace(opt["name"])
		if nodeName == "" && len(arg) > 0 {
			nodeName = arg[0]
		}
		parentURI, parentPath, err := f.commandCreateParent(ctx, opt)
		if err != nil {
			return nil, err
		}
		kind := "Album"
		if name == "create-folder" {
			kind = "Folder"
		}
		return f.createNode(ctx, parentURI, parentPath, kind, nodeName, opt["url_name"], opt["privacy"])
	case "copy-image", "move-image":
		srcRemote, dstRemote, err := commandImageTransferArgs(arg, opt)
		if err != nil {
			return nil, err
		}
		return f.copyOrMoveImageCommand(ctx, srcRemote, dstRemote, name == "move-image")
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

func (f *Fs) newObjectFromImage(remote string, image albumImage) *Object {
	return f.newObjectFromImageInAlbum(remote, image, f.albumURI, f.remoteFromImage(image))
}

func (f *Fs) newObjectFromImageInAlbum(remote string, image albumImage, albumURI, albumRemote string) *Object {
	size := image.OriginalSize
	if size == 0 {
		size = image.Size
	}
	if size == 0 {
		size = -1
	}
	modTime := parseSmugMugTime(image.LastUpdated)
	if modTime.IsZero() {
		modTime = parseSmugMugTime(image.Date)
	}
	imageURI := image.Uris["Image"].Uri
	if imageURI == "" {
		imageURI = imageURIFromAlbumImageURI(image.Uri)
	}
	return &Object{
		fs:            f,
		remote:        remote,
		albumURI:      albumURI,
		albumRemote:   albumRemote,
		size:          size,
		modTime:       modTime,
		contentType:   image.MimeType,
		title:         image.Title,
		caption:       image.Caption,
		keywords:      string(image.Keywords),
		hidden:        image.Hidden,
		latitude:      image.Latitude.ptr(),
		longitude:     image.Longitude.ptr(),
		altitude:      image.Altitude.ptr(),
		webURI:        image.WebUri,
		imageURI:      imageURI,
		albumImageURI: image.Uri,
		downloadURL:   image.ArchivedUri,
	}
}

func imageURIFromAlbumImageURI(uri string) string {
	const marker = "/image/"
	i := strings.LastIndex(uri, marker)
	if i < 0 {
		return uri
	}
	imageKey := strings.Trim(uri[i+len(marker):], "/")
	if imageKey == "" {
		return uri
	}
	return "/api/v2/image/" + imageKey
}

func (f *Fs) remoteFromImage(image albumImage) string {
	name := image.FileName
	if name == "" {
		name = image.Title
	}
	if name == "" {
		name = path.Base(image.Uri)
	}
	return cleanRemote(f.opt.Enc.ToStandardPath(name))
}

func (f *Fs) encodedRemote(remote string) string {
	fullRemote := cleanRemote(path.Join(f.root, remote))
	return f.opt.Enc.FromStandardPath(fullRemote)
}

func (f *Fs) doJSON(ctx context.Context, method, uri string, in, out any) error {
	var body []byte
	var err error
	if in != nil {
		body, err = json.Marshal(in)
		if err != nil {
			return err
		}
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		var reader io.Reader
		if body != nil {
			reader = bytes.NewReader(body)
		}
		req, err := http.NewRequestWithContext(ctx, method, f.apiURL(uri), reader)
		if err != nil {
			return false, err
		}
		req.Header.Set("Accept", "application/json")
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err = f.client.Do(req)
		retry, err := shouldRetry(ctx, resp, err)
		if retry && resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return retry, err
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseHTTPError(resp, respBody)
	}
	if out != nil && len(respBody) != 0 {
		err = json.Unmarshal(respBody, out)
		if err != nil {
			return fmt.Errorf("failed to decode SmugMug response: %w", err)
		}
	}
	return nil
}

func (f *Fs) apiURL(uri string) string {
	if strings.HasPrefix(uri, "http://") || strings.HasPrefix(uri, "https://") {
		return uri
	}
	if strings.HasPrefix(uri, "/") {
		return apiOrigin + uri
	}
	return apiOrigin + "/" + uri
}

func smugMugMetadataPatch(metadata fs.Metadata) (map[string]any, error) {
	patch := map[string]any{}
	for key, value := range metadata {
		switch strings.ToLower(key) {
		case "title":
			patch["Title"] = value
		case "caption":
			patch["Caption"] = value
		case "keywords":
			patch["Keywords"] = value
		case "hidden":
			hidden, err := strconv.ParseBool(strings.TrimSpace(value))
			if err != nil {
				return nil, fmt.Errorf("invalid hidden metadata value %q: %w", value, err)
			}
			patch["Hidden"] = hidden
		case "latitude":
			latitude, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid latitude metadata value %q: %w", value, err)
			}
			patch["Latitude"] = latitude
		case "longitude":
			longitude, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid longitude metadata value %q: %w", value, err)
			}
			patch["Longitude"] = longitude
		case "altitude":
			altitude, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
			if err != nil {
				return nil, fmt.Errorf("invalid altitude metadata value %q: %w", value, err)
			}
			patch["Altitude"] = altitude
		}
	}
	return patch, nil
}

func applySmugMugUploadMetadata(headers map[string]string, metadata fs.Metadata) error {
	patch, err := smugMugMetadataPatch(metadata)
	if err != nil {
		return err
	}
	for key, value := range patch {
		switch key {
		case "Title":
			headers["X-Smug-Title"] = value.(string)
		case "Caption":
			headers["X-Smug-Caption"] = value.(string)
		case "Keywords":
			headers["X-Smug-Keywords"] = value.(string)
		case "Hidden":
			headers["X-Smug-Hidden"] = strconv.FormatBool(value.(bool))
		case "Latitude":
			headers["X-Smug-Latitude"] = strconv.FormatFloat(value.(float64), 'f', -1, 64)
		case "Longitude":
			headers["X-Smug-Longitude"] = strconv.FormatFloat(value.(float64), 'f', -1, 64)
		case "Altitude":
			headers["X-Smug-Altitude"] = strconv.FormatFloat(value.(float64), 'f', -1, 64)
		}
	}
	return nil
}

func metadataFromOptions(ctx context.Context, dst fs.Fs, src fs.ObjectInfo, options []fs.OpenOption) (metadata fs.Metadata, err error) {
	if fs.GetConfig(ctx).Metadata {
		return fs.GetMetadataOptions(ctx, dst, src, options)
	}
	metadata.MergeOptions(options)
	return metadata, nil
}

// Fs returns read only access to the Fs that this object is part of.
func (o *Object) Fs() fs.Info {
	return o.fs
}

// String returns a description of the Object.
func (o *Object) String() string {
	return o.remote
}

// Remote returns the remote path.
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the selected checksum.
func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// ModTime returns the modification date of the file.
func (o *Object) ModTime(ctx context.Context) time.Time {
	if !o.modTime.IsZero() {
		return o.modTime
	}
	return time.Time(fs.GetConfig(ctx).DefaultTime)
}

// Size returns the size of the file.
func (o *Object) Size() int64 {
	return o.size
}

// Storable returns whether this object can be stored.
func (o *Object) Storable() bool {
	return true
}

// SetModTime sets the modification time.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// MimeType returns the content type of the object.
func (o *Object) MimeType(ctx context.Context) string {
	if o.contentType != "" {
		return o.contentType
	}
	if mt := mime.TypeByExtension(path.Ext(o.remote)); mt != "" {
		return mt
	}
	return "application/octet-stream"
}

func (o *Object) publicLink(ctx context.Context) (string, error) {
	if o.webURI != "" {
		return o.webURI, nil
	}
	uri := o.albumImageURI
	if uri == "" {
		uri = o.imageURI
	}
	if uri == "" {
		return "", fs.ErrorObjectNotFound
	}
	var result apiResponse
	if err := o.fs.doJSON(ctx, http.MethodGet, addQuery(uri, "_verbosity", "1"), nil, &result); err != nil {
		return "", err
	}
	var image albumImage
	switch {
	case len(result.Response.AlbumImage) > 0:
		image = result.Response.AlbumImage[0]
	case len(result.Response.Image) > 0:
		image = result.Response.Image[0]
	default:
		return "", errors.New("SmugMug image response did not include image metadata")
	}
	if image.WebUri == "" {
		return "", errors.New("SmugMug image response did not include a web URL")
	}
	o.webURI = image.WebUri
	return o.webURI, nil
}

// Metadata returns SmugMug image metadata.
func (o *Object) Metadata(ctx context.Context) (fs.Metadata, error) {
	return o.smugMugMetadata(), nil
}

func (o *Object) smugMugMetadata() fs.Metadata {
	var metadata fs.Metadata
	if o.title != "" {
		metadata.Set("title", o.title)
	}
	if o.caption != "" {
		metadata.Set("caption", o.caption)
	}
	if o.keywords != "" {
		metadata.Set("keywords", o.keywords)
	}
	if o.hidden != nil {
		metadata.Set("hidden", strconv.FormatBool(*o.hidden))
	}
	if o.latitude != nil {
		metadata.Set("latitude", strconv.FormatFloat(*o.latitude, 'f', -1, 64))
	}
	if o.longitude != nil {
		metadata.Set("longitude", strconv.FormatFloat(*o.longitude, 'f', -1, 64))
	}
	if o.altitude != nil {
		metadata.Set("altitude", strconv.FormatFloat(*o.altitude, 'f', -1, 64))
	}
	if len(metadata) == 0 {
		return nil
	}
	return metadata
}

func (o *Object) setMetadata(metadata fs.Metadata) {
	patch, err := smugMugMetadataPatch(metadata)
	if err != nil {
		return
	}
	o.applyMetadataPatch(patch)
}

// SetMetadata updates SmugMug image metadata.
func (o *Object) SetMetadata(ctx context.Context, metadata fs.Metadata) error {
	patch, err := smugMugMetadataPatch(metadata)
	if err != nil {
		return err
	}
	if len(patch) == 0 {
		return nil
	}
	uri := o.albumImageURI
	if uri == "" {
		uri = o.imageURI
	}
	if uri == "" {
		return fs.ErrorObjectNotFound
	}
	if err := o.fs.doJSON(ctx, http.MethodPatch, uri, patch, nil); err != nil {
		return err
	}
	o.applyMetadataPatch(patch)
	return nil
}

func (o *Object) applyMetadataPatch(patch map[string]any) {
	for key, value := range patch {
		switch key {
		case "Title":
			o.title = value.(string)
		case "Caption":
			o.caption = value.(string)
		case "Keywords":
			o.keywords = value.(string)
		case "Hidden":
			hidden := value.(bool)
			o.hidden = &hidden
		case "Latitude":
			latitude := value.(float64)
			o.latitude = &latitude
		case "Longitude":
			longitude := value.(float64)
			o.longitude = &longitude
		case "Altitude":
			altitude := value.(float64)
			o.altitude = &altitude
		}
	}
}

// Open opens the object for reading.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	downloadURL, err := o.getDownloadURL(ctx)
	if err != nil {
		return nil, err
	}
	var resp *http.Response
	err = o.fs.pacer.Call(func() (bool, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
		if err != nil {
			return false, err
		}
		for _, option := range options {
			key, value := option.Header()
			if key != "" && value != "" {
				req.Header.Set(key, value)
			}
			if option.Mandatory() {
				switch option.(type) {
				case *fs.RangeOption, *fs.SeekOption:
				default:
					fs.Logf(o, "Unsupported mandatory option: %v", option)
				}
			}
		}
		resp, err = o.fs.downloadClient.Do(req)
		retry, err := shouldRetry(ctx, resp, err)
		if retry && resp != nil && resp.Body != nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}
		return retry, err
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return nil, parseHTTPError(resp, body)
	}
	return resp.Body, nil
}

// Update uploads or replaces an object.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	if size < 0 {
		return errors.New("can't upload object of unknown size to SmugMug")
	}

	md5Base64, uploadBody, cleanup, err := o.prepareUpload(ctx, in, src)
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	var upload uploadResponse
	albumURI, albumRemote, err := o.fs.uploadTarget(ctx, src.Remote())
	if err != nil {
		return err
	}
	contentType := fs.MimeType(ctx, src)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	fileName := o.fs.opt.Enc.FromStandardPath(albumRemote)
	metadata, err := metadataFromOptions(ctx, o.fs, src, options)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"Content-MD5":         md5Base64,
		"Content-Type":        contentType,
		"X-Smug-AlbumUri":     albumURI,
		"X-Smug-FileName":     fileName,
		"X-Smug-ResponseType": "JSON",
		"X-Smug-Version":      "v2",
	}
	if err = applySmugMugUploadMetadata(headers, metadata); err != nil {
		return err
	}

	err = o.fs.upload(ctx, uploadBody, size, headers, o.imageURI, &upload)
	if err != nil {
		return err
	}
	if upload.Stat != "" && upload.Stat != "ok" {
		return fmt.Errorf("SmugMug upload failed: %s", upload.Message)
	}
	o.remote = src.Remote()
	o.albumURI = albumURI
	o.albumRemote = albumRemote
	o.size = size
	o.modTime = src.ModTime(ctx)
	o.contentType = contentType
	o.imageURI = upload.Image.ImageUri
	o.albumImageURI = upload.Image.AlbumImageUri
	o.setMetadata(metadata)
	return nil
}

func (o *Object) prepareUpload(ctx context.Context, in io.Reader, src fs.ObjectInfo) (
	md5Base64 string, out io.Reader, cleanup func(), err error,
) {
	md5Hex, hashErr := src.Hash(ctx, hash.MD5)
	if hashErr == nil && md5Hex != "" {
		md5Base64, err = md5HexToBase64(md5Hex)
		if err != nil {
			return "", nil, nil, err
		}
		return md5Base64, in, func() {}, nil
	}

	var wrap accounting.WrapFn
	in, wrap = accounting.UnWrap(in)
	md5Hex, out, cleanup, err = readMD5(in, src.Size(), int64(o.fs.opt.MD5MemoryLimit))
	if err != nil {
		return "", nil, cleanup, fmt.Errorf("failed to calculate upload MD5: %w", err)
	}
	md5Base64, err = md5HexToBase64(md5Hex)
	if err != nil {
		return "", nil, cleanup, err
	}
	return md5Base64, wrap(out), cleanup, nil
}

func (f *Fs) upload(
	ctx context.Context,
	in io.Reader,
	size int64,
	headers map[string]string,
	replaceURI string,
	out *uploadResponse,
) error {
	var resp *http.Response
	var tried bool
	err := f.pacer.Call(func() (bool, error) {
		if tried {
			if seeker, ok := in.(io.Seeker); ok {
				if _, err := seeker.Seek(0, io.SeekStart); err != nil {
					return false, err
				}
			} else {
				return false, errors.New("can't retry SmugMug upload with a non-seekable body")
			}
		}
		tried = true
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, in)
		if err != nil {
			return false, err
		}
		req.ContentLength = size
		for key, value := range headers {
			req.Header.Set(key, value)
		}
		if replaceURI != "" {
			req.Header.Set("X-Smug-ImageUri", replaceURI)
		}
		resp, err = f.client.Do(req)
		retry, err := shouldRetry(ctx, resp, err)
		if retry {
			if _, ok := in.(io.Seeker); !ok {
				if resp != nil {
					return false, nil
				}
				return false, err
			}
			if resp != nil && resp.Body != nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}
		return retry, err
	})
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return parseHTTPError(resp, body)
	}
	if len(body) != 0 {
		if err := json.Unmarshal(body, out); err != nil {
			return fmt.Errorf("failed to decode SmugMug upload response: %w", err)
		}
	}
	return nil
}

// Remove removes this object.
func (o *Object) Remove(ctx context.Context) error {
	uri := o.albumImageURI
	if uri == "" {
		uri = o.imageURI
	}
	if uri == "" {
		return fs.ErrorObjectNotFound
	}
	return o.fs.doJSON(ctx, http.MethodDelete, uri, nil, nil)
}

func (o *Object) getDownloadURL(ctx context.Context) (string, error) {
	if o.downloadURL != "" && strings.HasPrefix(o.downloadURL, "http") {
		return o.downloadURL, nil
	}
	uri := o.imageURI
	if uri == "" {
		return "", errors.New("SmugMug image URI is unknown")
	}
	if !strings.HasSuffix(uri, "!sizedetails") {
		uri += "!sizedetails"
	}
	var raw map[string]any
	if err := o.fs.doJSON(ctx, http.MethodGet, uri, nil, &raw); err != nil {
		return "", err
	}
	downloadURL := findDownloadURL(raw)
	if downloadURL == "" {
		return "", errors.New("SmugMug response did not include a downloadable media URL")
	}
	o.downloadURL = downloadURL
	return downloadURL, nil
}

func findDownloadURL(v any) string {
	preferred := []string{"OriginalUrl", "OriginalURL", "ArchivedUri", "LargestImageUrl", "LargestImageURL", "Url", "URL"}
	switch x := v.(type) {
	case map[string]any:
		for _, key := range preferred {
			if value, ok := x[key].(string); ok && strings.HasPrefix(value, "http") {
				return value
			}
		}
		keys := make([]string, 0, len(x))
		for key := range x {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			if value := findDownloadURL(x[key]); value != "" {
				return value
			}
		}
	case []any:
		for _, item := range x {
			if value := findDownloadURL(item); value != "" {
				return value
			}
		}
	}
	return ""
}

func requestToken(ctx context.Context, opt *Options) (token, secret string, err error) {
	values, err := oauthRequest(ctx, opt, http.MethodPost, requestTokenURL, "", "", map[string]string{
		"oauth_callback": "oob",
	})
	if err != nil {
		return "", "", err
	}
	return values.Get("oauth_token"), values.Get("oauth_token_secret"), nil
}

func accessToken(ctx context.Context, opt *Options, requestToken, requestSecret, verifier string) (token, secret string, err error) {
	if verifier == "" {
		return "", "", errors.New("verification code was empty")
	}
	values, err := oauthRequest(ctx, opt, http.MethodPost, accessTokenURL, requestToken, requestSecret, map[string]string{
		"oauth_verifier": verifier,
	})
	if err != nil {
		return "", "", err
	}
	return values.Get("oauth_token"), values.Get("oauth_token_secret"), nil
}

func oauthRequest(
	ctx context.Context,
	opt *Options,
	method, rawURL, token, tokenSecret string,
	extra map[string]string,
) (url.Values, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, nil)
	if err != nil {
		return nil, err
	}
	cred := oauthCredentials{
		consumerKey:    opt.APIKey,
		consumerSecret: opt.APISecret,
		token:          token,
		tokenSecret:    tokenSecret,
	}
	if err := signOAuth1(req, cred, extra); err != nil {
		return nil, err
	}
	resp, err := fshttp.NewClient(ctx).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, parseHTTPError(resp, body)
	}
	values, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, err
	}
	if values.Get("oauth_token") == "" || values.Get("oauth_token_secret") == "" {
		return nil, fmt.Errorf("SmugMug OAuth response missing token: %s", string(body))
	}
	return values, nil
}

func makeAuthorizeURL(token string) (string, error) {
	u, err := url.Parse(authorizeURL)
	if err != nil {
		return "", err
	}
	q := u.Query()
	q.Set("oauth_token", token)
	q.Set("Access", "Full")
	q.Set("Permissions", "Modify")
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (t *oauth1Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	clone := req.Clone(req.Context())
	clone.Header = req.Header.Clone()
	if err := signOAuth1(clone, t.cred, nil); err != nil {
		return nil, err
	}
	return base.RoundTrip(clone)
}

func signOAuth1(req *http.Request, cred oauthCredentials, extra map[string]string) error {
	if cred.consumerKey == "" || cred.consumerSecret == "" {
		return errors.New("missing OAuth consumer credentials")
	}
	oauthParams := map[string]string{
		"oauth_consumer_key":     cred.consumerKey,
		"oauth_nonce":            oauthNonce(),
		"oauth_signature_method": "HMAC-SHA1",
		"oauth_timestamp":        strconv.FormatInt(time.Now().Unix(), 10),
		"oauth_version":          "1.0",
	}
	if cred.token != "" {
		oauthParams["oauth_token"] = cred.token
	}
	for key, value := range extra {
		oauthParams[key] = value
	}

	signatureParams := make(url.Values)
	for key, values := range req.URL.Query() {
		for _, value := range values {
			signatureParams.Add(key, value)
		}
	}
	for key, value := range oauthParams {
		signatureParams.Add(key, value)
	}

	baseURL := *req.URL
	baseURL.RawQuery = ""
	baseURL.Fragment = ""
	baseString := strings.Join([]string{
		strings.ToUpper(req.Method),
		percentEncode(baseURL.String()),
		percentEncode(normalizeParams(signatureParams)),
	}, "&")
	signingKey := percentEncode(cred.consumerSecret) + "&" + percentEncode(cred.tokenSecret)
	mac := hmac.New(sha1.New, []byte(signingKey))
	_, _ = mac.Write([]byte(baseString))
	oauthParams["oauth_signature"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))

	req.Header.Set("Authorization", oauthAuthorizationHeader(oauthParams))
	return nil
}

func oauthNonce() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:])
	}
	return strconv.FormatInt(time.Now().UnixNano(), 16)
}

func normalizeParams(values url.Values) string {
	type pair struct {
		key   string
		value string
	}
	var pairs []pair
	for key, vals := range values {
		for _, value := range vals {
			pairs = append(pairs, pair{key: percentEncode(key), value: percentEncode(value)})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].key == pairs[j].key {
			return pairs[i].value < pairs[j].value
		}
		return pairs[i].key < pairs[j].key
	})
	parts := make([]string, len(pairs))
	for i, pair := range pairs {
		parts[i] = pair.key + "=" + pair.value
	}
	return strings.Join(parts, "&")
}

func oauthAuthorizationHeader(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		if !strings.HasPrefix(key, "oauth_") {
			continue
		}
		parts = append(parts, percentEncode(key)+`="`+percentEncode(params[key])+`"`)
	}
	return "OAuth " + strings.Join(parts, ", ")
}

func percentEncode(in string) string {
	return strings.ReplaceAll(url.QueryEscape(in), "+", "%20")
}

func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if resp != nil && resp.Header.Get("Retry-After") != "" {
		if duration, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			return true, pacer.RetryAfterError(err, duration)
		}
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

func parseRetryAfter(value string, now time.Time) (time.Duration, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			seconds = 0
		}
		return time.Duration(seconds) * time.Second, true
	}
	when, err := http.ParseTime(value)
	if err != nil {
		return 0, false
	}
	if when.Before(now) {
		return 0, true
	}
	return when.Sub(now), true
}

func parseHTTPError(resp *http.Response, body []byte) error {
	if len(body) == 0 {
		return fmt.Errorf("SmugMug error %s", resp.Status)
	}
	var apiErr apiResponse
	if json.Unmarshal(body, &apiErr) == nil && apiErr.Message != "" {
		return fmt.Errorf("SmugMug error %s: %s", resp.Status, apiErr.Message)
	}
	return fmt.Errorf("SmugMug error %s: %s", resp.Status, strings.TrimSpace(string(body)))
}

func (f *Fs) resolveAlbumURI(ctx context.Context, in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", errors.New("album_uri is required")
	}

	if strings.HasPrefix(in, "http://") || strings.HasPrefix(in, "https://") {
		u, err := url.Parse(in)
		if err != nil {
			return "", err
		}
		if uri, err := f.resolveAlbumWebURL(ctx, u); err == nil {
			return uri, nil
		}
		return f.resolveAlbumPath(ctx, u.Hostname(), albumPathFromURLPath(u.Path))
	}

	if strings.HasPrefix(in, "/api/v2/album/") ||
		strings.HasPrefix(in, "/album/") ||
		!strings.Contains(in, "/") {
		return normalizeAlbumURI(in)
	}

	return f.resolveAlbumPath(ctx, "", albumPathFromURLPath(in))
}

func (f *Fs) resolveAlbumWebURL(ctx context.Context, u *url.URL) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", err
	}
	resp, err := fshttp.NewClient(ctx).Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return "", fmt.Errorf("SmugMug page lookup failed: %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return "", err
	}
	page := normalizeSmugMugPageText(string(body))
	if uri := firstAlbumURI(page); uri != "" {
		return uri, nil
	}
	if nodeID := firstMatch(nodeIDRe, page); nodeID != "" {
		if uri, err := f.resolveNodeAlbumURI(ctx, nodeID); err == nil {
			return uri, nil
		}
	}
	if nickname := firstMatch(nickNameRe, page); nickname != "" {
		return f.resolveAlbumPath(ctx, nickname, albumPathFromURLPath(u.Path))
	}
	return "", errors.New("could not find SmugMug album metadata in page")
}

func (f *Fs) resolveAlbumPath(ctx context.Context, hostOrNick, albumPath string) (string, error) {
	nickname := hostOrNick
	if strings.HasSuffix(nickname, ".smugmug.com") {
		nickname = strings.TrimSuffix(nickname, ".smugmug.com")
	}
	if strings.Contains(nickname, ".") || nickname == "" {
		return "", errors.New("could not determine SmugMug nickname for album path")
	}
	q := url.Values{}
	q.Set("urlpath", albumPath)
	q.Set("_verbosity", "1")
	var result apiResponse
	uri := "/api/v2/user/" + url.PathEscape(nickname) + "!urlpathlookup?" + q.Encode()
	if err := f.doJSON(ctx, http.MethodGet, uri, nil, &result); err != nil {
		return "", err
	}
	if result.Response.Album != nil && result.Response.Album.Uri != "" {
		return result.Response.Album.Uri, nil
	}
	if result.Response.Node != nil {
		if albumLink, ok := result.Response.Node.Uris["Album"]; ok && albumLink.Uri != "" {
			return albumLink.Uri, nil
		}
	}
	return "", fmt.Errorf("SmugMug path %q did not resolve to an album", albumPath)
}

func (f *Fs) resolveNodeAlbumURI(ctx context.Context, nodeID string) (string, error) {
	var result apiResponse
	err := f.doJSON(ctx, http.MethodGet, "/api/v2/node/"+url.PathEscape(nodeID)+"?_verbosity=1", nil, &result)
	if err != nil {
		return "", err
	}
	if result.Response.Node != nil {
		if albumLink, ok := result.Response.Node.Uris["Album"]; ok && albumLink.Uri != "" {
			return albumLink.Uri, nil
		}
	}
	return "", fmt.Errorf("SmugMug node %q is not an album", nodeID)
}

func normalizeSmugMugPageText(in string) string {
	in = html.UnescapeString(in)
	in = strings.ReplaceAll(in, `\/`, `/`)
	in = strings.ReplaceAll(in, `\u002F`, `/`)
	in = strings.ReplaceAll(in, `\u002f`, `/`)
	return in
}

func firstAlbumURI(in string) string {
	return albumURIRe.FindString(in)
}

func firstMatch(re *regexp.Regexp, in string) string {
	match := re.FindStringSubmatch(in)
	if len(match) < 2 {
		return ""
	}
	return match[1]
}

func albumPathFromURLPath(in string) string {
	parts := strings.Split(strings.Trim(in, "/"), "/")
	if len(parts) == 0 {
		return "/"
	}
	for i, part := range parts {
		if strings.HasPrefix(part, "i-") {
			parts = parts[:i]
			break
		}
	}
	return "/" + strings.Join(parts, "/")
}

func normalizeAlbumURI(in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", errors.New("album_uri is required")
	}
	if strings.HasPrefix(in, "http://") || strings.HasPrefix(in, "https://") {
		u, err := url.Parse(in)
		if err != nil {
			return "", err
		}
		in = u.Path
	}
	in = "/" + strings.Trim(in, "/")
	switch {
	case strings.HasPrefix(in, "/api/v2/album/"):
	case strings.HasPrefix(in, "/album/"):
		in = "/api/v2" + in
	default:
		in = "/api/v2/album/" + strings.Trim(in, "/")
	}
	if !strings.HasPrefix(in, "/api/v2/album/") || strings.TrimPrefix(in, "/api/v2/album/") == "" {
		return "", fmt.Errorf("invalid SmugMug album URI %q", in)
	}
	return in, nil
}

func normalizeNodeURI(in string) (string, error) {
	in = strings.TrimSpace(in)
	if in == "" {
		return "", errors.New("node URI is required")
	}
	if strings.HasPrefix(in, "http://") || strings.HasPrefix(in, "https://") {
		u, err := url.Parse(in)
		if err != nil {
			return "", err
		}
		in = u.Path
	}
	in = "/" + strings.Trim(in, "/")
	switch {
	case strings.HasPrefix(in, "/api/v2/node/"):
	case strings.HasPrefix(in, "/node/"):
		in = "/api/v2" + in
	default:
		if strings.Contains(strings.Trim(in, "/"), "/") {
			return "", fmt.Errorf("invalid SmugMug node URI %q", in)
		}
		in = "/api/v2/node/" + strings.Trim(in, "/")
	}
	if !strings.HasPrefix(in, "/api/v2/node/") || strings.TrimPrefix(in, "/api/v2/node/") == "" {
		return "", fmt.Errorf("invalid SmugMug node URI %q", in)
	}
	return in, nil
}

func addQuery(uri, key, value string) string {
	sep := "?"
	if strings.Contains(uri, "?") {
		sep = "&"
	}
	return uri + sep + url.QueryEscape(key) + "=" + url.QueryEscape(value)
}

func parseBoolOption(opt map[string]string, key string) (bool, error) {
	value, ok := opt[key]
	if !ok || strings.TrimSpace(value) == "" {
		return false, nil
	}
	out, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s value %q: %w", key, value, err)
	}
	return out, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func parseSmugMugTime(in string) time.Time {
	if in == "" {
		return time.Time{}
	}
	t, err := time.Parse(time.RFC3339, in)
	if err == nil {
		return t
	}
	return time.Time{}
}

func cleanRemote(remote string) string {
	remote = strings.Trim(path.Clean(remote), "/")
	if remote == "." {
		return ""
	}
	return remote
}

func md5HexToBase64(in string) (string, error) {
	sum, err := hex.DecodeString(in)
	if err != nil {
		return "", fmt.Errorf("failed to decode MD5 %q: %w", in, err)
	}
	return base64.StdEncoding.EncodeToString(sum), nil
}

func readMD5(in io.Reader, size, threshold int64) (md5sum string, out io.Reader, cleanup func(), err error) {
	md5Hasher := md5.New()
	teeReader := io.TeeReader(in, md5Hasher)
	cleanup = func() {}
	if size > threshold {
		var tempFile *os.File
		tempFile, err = os.CreateTemp("", cacheFilePrefix)
		if err != nil {
			return
		}
		cleanup = func() {
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name())
		}
		if _, err = io.Copy(tempFile, teeReader); err != nil {
			return
		}
		if _, err = tempFile.Seek(0, io.SeekStart); err != nil {
			return
		}
		out = tempFile
	} else {
		var data []byte
		data, err = io.ReadAll(teeReader)
		if err != nil {
			return
		}
		out = bytes.NewReader(data)
	}
	return hex.EncodeToString(md5Hasher.Sum(nil)), out, cleanup, nil
}

var (
	_ fs.Fs            = (*Fs)(nil)
	_ fs.Commander     = (*Fs)(nil)
	_ fs.UserInfoer    = (*Fs)(nil)
	_ fs.PublicLinker  = (*Fs)(nil)
	_ fs.Object        = (*Object)(nil)
	_ fs.MimeTyper     = (*Object)(nil)
	_ fs.Metadataer    = (*Object)(nil)
	_ fs.SetMetadataer = (*Object)(nil)
)
