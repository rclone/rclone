package putio

import (
	"log"
	"regexp"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/oauthutil"
	"golang.org/x/oauth2"
)

// Constants
const (
	rcloneClientID             = "4131"
	rcloneObscuredClientSecret = "cMwrjWVmrHZp3gf1ZpCrlyGAmPpB-YY5BbVnO1fj-G9evcd8"
	minSleep                   = 10 * time.Millisecond
	maxSleep                   = 2 * time.Second
	decayConstant              = 2 // bigger for slower decay, exponential
	defaultChunkSize           = 48 * fs.MebiByte
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
		Config: func(name string, m configmap.Mapper) {
			err := oauthutil.ConfigNoOffline("putio", name, m, putioConfig)
			if err != nil {
				log.Fatalf("Failed to configure token: %v", err)
			}
		},
	})
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
