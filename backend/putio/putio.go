package putio

import (
	"context"
	"regexp"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"golang.org/x/oauth2"
)

/*
// TestPutio
stringNeedsEscaping = []rune{
	'/', '\x00'
}
maxFileLength = 255
canWriteUnnormalized = true
canReadUnnormalized   = true
canReadRenormalized   = true
canStream = false
*/

// Constants
const (
	rcloneClientID             = "4131"
	rcloneObscuredClientSecret = "cMwrjWVmrHZp3gf1ZpCrlyGAmPpB-YY5BbVnO1fj-G9evcd8"
	minSleep                   = 10 * time.Millisecond
	maxSleep                   = 2 * time.Second
	decayConstant              = 1 // bigger for slower decay, exponential
	defaultChunkSize           = 48 * fs.Mebi
	defaultRateLimitSleep      = 60 * time.Second
)

var (
	// Description of how to auth for this app
	putioConfig = &oauth2.Config{
		Scopes: []string{},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://api.put.io/v2/oauth2/authenticate",
			TokenURL: "https://api.put.io/v2/oauth2/access_token",
		},
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneObscuredClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
	// A regexp matching path names for ignoring unnecessary files
	ignoredFiles = regexp.MustCompile(`(?i)(^|/)(desktop\.ini|thumbs\.db|\.ds_store|icon\r)$`)
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "putio",
		Description: "Put.io",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			return oauthutil.ConfigOut("", &oauthutil.Options{
				OAuth2Config: putioConfig,
				NoOffline:    true,
			})
		},
		Options: []fs.Option{{
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Note that \ is renamed to -
			//
			// Encode invalid UTF-8 bytes as json doesn't handle them properly.
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Enc encoder.MultiEncoder `config:"encoding"`
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.CleanUpper      = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
