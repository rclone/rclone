// Package zoho provides an interface to the Zoho Workdrive
// storage system.
package zoho

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/random"

	"github.com/rclone/rclone/backend/zoho/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

const (
	rcloneClientID              = "1000.46MXF275FM2XV7QCHX5A7K3LGME66B"
	rcloneEncryptedClientSecret = "U-2gxclZQBcOG9NPhjiXAhj-f0uQ137D0zar8YyNHXHkQZlTeSpIOQfmCb4oSpvosJp_SJLXmLLeUA"
	configRootID                = "root_folder_id"
	// minSleep is the pacer's minimum delay between calls when --zoho-tpslimit
	// is 0 (cap disabled); a small floor always remains because backoff and
	// Retry-After still apply.
	minSleep = 10 * time.Millisecond

	defaultUploadCutoff = 10 * 1024 * 1024 // 10 MiB

	// defaultTPSLimit is the sustainable refill rate (API calls/second) of
	// Zoho WorkDrive's throttle, measured live. Going faster drains the token
	// bucket and then triggers long (~119s) 429 Retry-After stalls (see
	// --zoho-tpslimit).
	defaultTPSLimit = 6.0
	// defaultTPSLimitBurst is the token-bucket capacity; keep at 1 as burst > 1
	// caused synchronized 429 clusters.
	defaultTPSLimitBurst = 1

	// defaultListFolderWindow / defaultListFolderLimit / defaultListFolderBurst
	// configure the per-folder listing limiter; folderListLimiter documents the
	// measured throttle model. Zoho allows ~19 listings of one folder in any
	// rolling ~60s window and the 20th returns F7008 (measured live 2026-07-05:
	// every trip landed exactly on the 20th listing inside 60s, every clean run
	// stayed at <=19 - including a deliberate over-limit probe tripping at #20).
	// The limiter guarantees at most limit listings per window: each window
	// starts with burst listings passing immediately (the burst RE-ARMS at every
	// window boundary) and the remaining limit-burst are paced window/(limit-burst)
	// apart, while a sliding log of the last limit grants enforces the cap across
	// window boundaries and idle resumes. burst never raises the per-window total -
	// it only sets how many listings may go back-to-back. Bursts of 4, 5 and 6
	// under this cap all ran clean live; 6 is the largest validated, hence the
	// default. Only repeated same-folder listings are paced; set limit to 0 to
	// disable entirely.
	defaultListFolderWindow = fs.Duration(60 * time.Second)
	defaultListFolderLimit  = 19 // 0 = disabled
	defaultListFolderBurst  = 6
	// folderListLimitersMaxEntries caps the per-folder limiter map against unbounded
	// growth on very long-lived processes.
	folderListLimitersMaxEntries = 100000

	// retryAfterMargin adds a small extra delay after Zoho's Retry-After value.
	// This gives rate-limit tokens time to refill and helps avoid another 429.
	retryAfterMargin = 1 * time.Second
)

// Globals
var (
	// Description of how to auth for this app
	oauthConfig = &oauthutil.Config{
		Scopes: []string{
			"aaaserver.profile.read",
			"WorkDrive.team.READ",
			"WorkDrive.workspace.READ",
			"WorkDrive.files.ALL",
			"ZohoFiles.files.ALL",
		},

		AuthURL:      "https://accounts.zoho.eu/oauth/v2/auth",
		TokenURL:     "https://accounts.zoho.eu/oauth/v2/token",
		AuthStyle:    oauth2.AuthStyleInParams,
		ClientID:     rcloneClientID,
		ClientSecret: obscure.MustReveal(rcloneEncryptedClientSecret),
		RedirectURL:  oauthutil.RedirectLocalhostURL,
	}
	rootURL     = "https://workdrive.zoho.eu/api/v1"
	downloadURL = "https://download.zoho.eu/v1/workdrive"
	uploadURL   = "http://upload.zoho.eu/workdrive-api/v1/"
	accountsURL = "https://accounts.zoho.eu"
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "zoho",
		Description: "Zoho",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			// Need to setup region before configuring oauth
			err := setupRegion(m)
			if err != nil {
				return nil, err
			}
			getSrvs := func() (authSrv, apiSrv *rest.Client, err error) {
				oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
				if err != nil {
					return nil, nil, fmt.Errorf("failed to load OAuth client: %w", err)
				}
				authSrv = rest.NewClient(oAuthClient).SetRoot(accountsURL)
				apiSrv = rest.NewClient(oAuthClient).SetRoot(rootURL)
				return authSrv, apiSrv, nil
			}

			switch config.State {
			case "":
				return oauthutil.ConfigOut("type", &oauthutil.Options{
					OAuth2Config: oauthConfig,
					// No refresh token unless ApprovalForce is set
					OAuth2Opts: []oauth2.AuthCodeOption{oauth2.ApprovalForce},
				})
			case "type":
				// We need to rewrite the token type to "Zoho-oauthtoken" because Zoho wants
				// it's own custom type
				token, err := oauthutil.GetToken(name, m)
				if err != nil {
					return nil, fmt.Errorf("failed to read token: %w", err)
				}
				if token.TokenType != "Zoho-oauthtoken" {
					token.TokenType = "Zoho-oauthtoken"
					err = oauthutil.PutToken(name, m, token, false)
					if err != nil {
						return nil, fmt.Errorf("failed to configure token: %w", err)
					}
				}

				// If a root_folder_id is already set (from config, or a previous
				// setup) don't overwrite it on update/reconnect unless the user
				// asks to, mirroring how the drive backend gates its team drive id.
				if rootID, _ := m.Get(configRootID); rootID != "" {
					return fs.ConfigConfirm("root_change", false, "config_change_root", fmt.Sprintf("Change current root folder id %q?\n", rootID))
				}
				return fs.ConfigGoto("select_edition")
			case "root_change":
				if config.Result == "false" {
					// Keep the existing root_folder_id; the token has already been refreshed.
					return nil, nil
				}
				return fs.ConfigGoto("select_edition")
			case "select_edition":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				userInfo, err := getUserInfo(ctx, apiSrv)
				if err != nil {
					return nil, err
				}
				// If personal Edition only one private Space is available. Directly configure that.
				if userInfo.Data.Attributes.Edition == "PERSONAL" {
					return fs.ConfigResult("private_space", userInfo.Data.ID)
				}
				// Otherwise go to team selection
				return fs.ConfigResult("team", userInfo.Data.ID)
			case "private_space":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				workspaces, err := getPrivateSpaces(ctx, config.Result, apiSrv)
				if err != nil {
					return nil, err
				}
				return fs.ConfigChoose("workspace_end", "config_workspace", "Workspace ID", len(workspaces), func(i int) (string, string) {
					workspace := workspaces[i]
					return workspace.ID, workspace.Name
				})
			case "team":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}

				// Get the teams
				teams, err := listTeams(ctx, config.Result, apiSrv)
				if err != nil {
					return nil, err
				}
				return fs.ConfigChoose("workspace", "config_team_drive_id", "Team Drive ID", len(teams), func(i int) (string, string) {
					team := teams[i]
					return team.ID, team.Attributes.Name
				})
			case "workspace":
				_, apiSrv, err := getSrvs()
				if err != nil {
					return nil, err
				}
				teamID := config.Result
				workspaces, err := listWorkspaces(ctx, teamID, apiSrv)
				if err != nil {
					return nil, err
				}
				currentTeamInfo, err := getCurrentTeamInfo(ctx, teamID, apiSrv)
				if err != nil {
					return nil, err
				}
				privateSpaces, err := getPrivateSpaces(ctx, currentTeamInfo.Data.ID, apiSrv)
				if err != nil {
					return nil, err
				}
				workspaces = append(workspaces, privateSpaces...)

				return fs.ConfigChoose("workspace_end", "config_workspace", "Workspace ID", len(workspaces), func(i int) (string, string) {
					workspace := workspaces[i]
					return workspace.ID, workspace.Name
				})
			case "workspace_end":
				workspaceID := config.Result
				m.Set(configRootID, workspaceID)
				return nil, nil
			}
			return nil, fmt.Errorf("unknown state %q", config.State)
		},
		Options: append(oauthutil.SharedOptions, []fs.Option{{
			Name: "region",
			Help: `Zoho region to connect to.

You'll have to use the region your organization is registered in. If
not sure use the same top level domain as you connect to in your
browser.`,
			Examples: []fs.OptionExample{{
				Value: "com",
				Help:  "United states / Global",
			}, {
				Value: "eu",
				Help:  "Europe",
			}, {
				Value: "in",
				Help:  "India",
			}, {
				Value: "jp",
				Help:  "Japan",
			}, {
				Value: "com.cn",
				Help:  "China",
			}, {
				Value: "com.au",
				Help:  "Australia",
			}},
		}, {
			Name:      "root_folder_id",
			Help:      "ID of the root folder.\n\nLeave blank normally.\n\nFill in to make rclone use a non root folder as its starting point.",
			Advanced:  true,
			Sensitive: true,
		}, {
			Name:     "upload_cutoff",
			Help:     "Cutoff for switching to large file upload api (>= 10 MiB).",
			Default:  fs.SizeSuffix(defaultUploadCutoff),
			Advanced: true,
		}, {
			Name: "tpslimit",
			Help: `Max number of API transactions per second.

Zoho WorkDrive rate limits its API and returns HTTP 429 (error F7008,
"Request rate limit exceeded") when called too quickly, so the data API
calls (list, upload, download, copy, move, delete) are paced to this rate.

Set to 0 to disable the cap, matching the global --tpslimit; pacing still
can't be turned off entirely because backoff and Retry-After always apply.

The default of 6 is a safe sustainable rate. Higher values can trigger long
429 Retry-After stalls that make throughput WORSE, so raise it only if your
account tolerates more.`,
			Default:  defaultTPSLimit,
			Advanced: true,
		}, {
			Name: "tpslimit_burst",
			Help: `Number of API calls to allow back-to-back without sleeping, for --zoho-tpslimit.

This is the token-bucket capacity. Keep at 1 for Zoho: a burst > 1
lets several calls fire at once after an idle gap, which can trigger
synchronized clusters of 429 errors.`,
			Default:  defaultTPSLimitBurst,
			Advanced: true,
		}, {
			Name: "list_folder_limit",
			Help: `Max listings of the SAME folder allowed per --zoho-list-folder-window.

Zoho WorkDrive rate limits its listing API (GET files/{id}/files) PER
folder, independently of --zoho-tpslimit: listing one folder too often in a
short time returns HTTP 429 (error F7008) with a multi-minute Retry-After
penalty, which a tight polling loop can hit even at a low overall rate.
Measured live, Zoho allows ~19 listings of one folder in any rolling ~60s
window and the 20th fails, which the defaults (19 per 60s) model exactly.

This is a true per-window cap for any traffic pattern: each window starts
with --zoho-list-folder-burst listings passing back-to-back (the burst
re-arms at every window boundary) and the rest are spaced
--zoho-list-folder-window/(limit - burst) apart (the defaults give ~4.6s),
while a sliding log of recent listings enforces the cap across window
boundaries. 0 disables the limiter. Only REPEATED listings of one folder are
delayed; different folders, or a folder listed fewer than
--zoho-list-folder-burst times, never are.

A HIGHER value means MORE listings per window, not more safety: raising it
above 19 trips F7008. Lower it for a wider margin at the cost of listing
responsiveness.`,
			Default:  defaultListFolderLimit,
			Advanced: true,
		}, {
			Name: "list_folder_window",
			Help: `The window for --zoho-list-folder-limit.

The default of 60s (shown as 1m0s) matches Zoho's real sliding window: at
most --zoho-list-folder-limit listings of one folder are allowed in any
window of this length. A bare number is parsed as seconds ("60" = "60s").

Widen it (or lower the limit) for a bigger safety margin; the sustained
spacing between same-folder listings is window/(limit - burst).`,
			Default:  defaultListFolderWindow,
			Advanced: true,
		}, {
			Name: "list_folder_burst",
			Help: `Same-folder listings allowed back-to-back before --zoho-list-folder-limit paces them.

The burst is carved out of --zoho-list-folder-limit, so raising it never
raises the per-window total: this many listings may fire immediately and the
remaining limit - burst are spaced window/(limit - burst) apart. The burst
RE-ARMS at every window boundary, so sustained re-listing gets a fresh burst
each window while a sliding log of recent listings still enforces the
per-window cap. A folder listed only a handful of times (the common case - a
sync re-listing one directory a few times then moving on) never waits.

The default 6 is the largest burst validated live under the default 19-per-60s
cap (bursts of 4, 5 and 6 all ran clean; an over-cap probe tripped F7008
exactly at the 20th listing in a window). Keep it well below ~15 - Zoho also
has an instantaneous back-to-back cap around 15-16 regardless of the window.
Set to 1 to pace from the second listing. Values >= the limit are clamped to
limit - 1.`,
			Default:  defaultListFolderBurst,
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.EncodeZero |
				encoder.EncodeCtl |
				encoder.EncodeDel |
				encoder.EncodeInvalidUtf8),
		}}...),
	})
}

// Options defines the configuration for this backend
type Options struct {
	UploadCutoff     fs.SizeSuffix        `config:"upload_cutoff"`
	RootFolderID     string               `config:"root_folder_id"`
	Region           string               `config:"region"`
	TPSLimit         float64              `config:"tpslimit"`
	TPSLimitBurst    int                  `config:"tpslimit_burst"`
	ListFolderLimit  int                  `config:"list_folder_limit"`
	ListFolderWindow fs.Duration          `config:"list_folder_window"`
	ListFolderBurst  int                  `config:"list_folder_burst"`
	Enc              encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote workdrive
type Fs struct {
	name        string             // name of this remote
	root        string             // the path we are working on
	opt         Options            // parsed options
	features    *fs.Features       // optional features
	srv         *rest.Client       // the connection to the server
	downloadsrv *rest.Client       // the connection to the download server
	uploadsrv   *rest.Client       // the connection to the upload server
	dirCache    *dircache.DirCache // Map of directory path to directory id
	pacer       *fs.Pacer          // pacer for API calls
	throttle    *throttleState     // once-per-episode 429 logging state (shared across Fs copies)
}

// throttleState tracks 429 throttling for once-per-episode logging (see
// logThrottle). It sits behind a pointer so the "assume it is a file" shallow
// copy of Fs shares one state; both fields are atomic, so it is lock-free. An
// "episode" is a run of 429s with no recovery (a success after the penalty
// window clears) between them.
type throttleState struct {
	penaltyUntilNano atomic.Int64 // unix-nanos the current 429 penalty should clear
	progress         atomic.Bool  // a call succeeded after the last penalty window
}

// folderListLimiters holds the per-folder-id listing rate limiters and the time
// each folder was last listed. One instance is shared process-wide (folderListLimiterRegistry)
// rather than per-*Fs because Zoho keys its listing throttle to the folder id:
// a per-Fs limiter would let each new Fs re-list a shared ancestor with a fresh,
// full-burst limiter, draining the bucket -> F7008.
type folderListLimiters struct {
	mu        sync.Mutex
	limiters  map[string]*folderListLimiterEntry
	lastSweep time.Time
}

// folderListLimiterRegistry is the process-wide registry of per-folder listing
// limiters, shared by every *Fs and keyed by region+folder-id so entries never
// collide across accounts or regions.
var folderListLimiterRegistry = &folderListLimiters{limiters: make(map[string]*folderListLimiterEntry)}

// folderListLimiterEntry is a per-folder listing rate limiter plus the time the folder
// was last listed, used for idle eviction.
type folderListLimiterEntry struct {
	limiter    *folderWindowLimiter
	lastListed time.Time
}

// folderWindowLimiter shapes same-folder listings to at most limit per window
// with a burst re-armed at each window boundary. Two layers:
//
// Shape (fixed window): a grid anchored at the first request advances in whole
// windows; each window's first burst grants pass immediately and the remaining
// limit-burst are spaced interval = window/(limit-burst) apart, so under
// continuous demand every window starts with a fast burst - the flow the
// options promise.
//
// Safety (sliding log): grants keeps the last limit grant times; a new grant
// is also forced to be >= grants[len-limit] + window + margin, so no rolling
// window ever sees more than limit grants even across grid boundaries or on
// resume-after-idle, where the fixed-window shape alone could cluster a
// boundary burst too close to earlier grants.
type folderWindowLimiter struct {
	mu          sync.Mutex
	window      time.Duration // Zoho's per-folder window (--zoho-list-folder-window)
	limit       int           // window budget AND sliding cap (--zoho-list-folder-limit)
	burst       int           // grants passed immediately at each window start
	interval    time.Duration // spacing of the paced phase: window/(limit-burst)
	windowStart time.Time     // fixed grid anchor; zero until the first grant
	used        int           // grants handed out in the current window
	lastGrant   time.Time     // time of the most recent grant
	grants      []time.Time   // last <=limit grant times (sliding safety log)
}

// folderListSafetyMargin is added on top of the sliding-log wait so a grant
// lands strictly after the matching old grant has left Zoho's window.
const folderListSafetyMargin = 500 * time.Millisecond

// reserve commits the next grant at or after now and returns its time.
// It converges in at most a few passes: rolling the grid forward resets the
// window budget, and the sliding log can push the candidate at most once more.
func (l *folderWindowLimiter) reserve(now time.Time) time.Time {
	l.mu.Lock()
	defer l.mu.Unlock()
	// Never let a grant move backwards, even if concurrent callers observe
	// time.Now() out of order: the sliding safety log below indexes grants as
	// an ordered history, so out-of-order grant times would corrupt the cap.
	if now.Before(l.lastGrant) {
		now = l.lastGrant
	}
	if l.windowStart.IsZero() {
		l.windowStart = now
	}
	cand := now
	for range 4 {
		// Roll the fixed grid forward so cand lies in the current window,
		// re-arming the burst for the new window.
		if d := cand.Sub(l.windowStart); d >= l.window {
			l.windowStart = l.windowStart.Add(l.window * (d / l.window))
			l.used = 0
		}
		next := cand
		switch {
		case l.used < l.burst:
			// window-start burst passes immediately
		case l.used < l.limit:
			if t := l.lastGrant.Add(l.interval); t.After(next) {
				next = t
			}
		default:
			// window budget exhausted - wait for the next window's burst
			next = l.windowStart.Add(l.window)
		}
		// Sliding safety: never more than limit grants in any rolling window.
		if len(l.grants) >= l.limit {
			if t := l.grants[len(l.grants)-l.limit].Add(l.window + folderListSafetyMargin); t.After(next) {
				next = t
			}
		}
		if next.Equal(cand) {
			break
		}
		cand = next
	}
	l.used++
	l.lastGrant = cand
	if len(l.grants) < l.limit {
		l.grants = append(l.grants, cand)
	} else {
		copy(l.grants, l.grants[1:])
		l.grants[len(l.grants)-1] = cand
	}
	return cand
}

// Wait blocks until the limiter grants a listing or ctx is cancelled. A grant
// reserved by a cancelled Wait is not returned - erring on the safe side, it
// just leaves a little of the window budget unused.
func (l *folderWindowLimiter) Wait(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	d := time.Until(l.reserve(time.Now()))
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Object describes a Zoho WorkDrive object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	id          string    // ID of the object
}

// ------------------------------------------------------------

func setupRegion(m configmap.Mapper) error {
	region, ok := m.Get("region")
	if !ok || region == "" {
		return errors.New("no region set")
	}
	rootURL = fmt.Sprintf("https://workdrive.zoho.%s/api/v1", region)
	downloadURL = fmt.Sprintf("https://download.zoho.%s/v1/workdrive", region)
	uploadURL = fmt.Sprintf("https://upload.zoho.%s/workdrive-api/v1", region)
	accountsURL = fmt.Sprintf("https://accounts.zoho.%s", region)
	oauthConfig.AuthURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/auth", region)
	oauthConfig.TokenURL = fmt.Sprintf("https://accounts.zoho.%s/oauth/v2/token", region)
	return nil
}

// ------------------------------------------------------------

type workspaceInfo struct {
	ID   string
	Name string
}

func getUserInfo(ctx context.Context, srv *rest.Client) (*api.UserInfoResponse, error) {
	var userInfo api.UserInfoResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/me",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &userInfo)
	if err != nil {
		return nil, err
	}
	return &userInfo, nil
}

func getCurrentTeamInfo(ctx context.Context, teamID string, srv *rest.Client) (*api.CurrentTeamInfo, error) {
	var currentTeamInfo api.CurrentTeamInfo
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/teams/" + teamID + "/currentuser",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &currentTeamInfo)
	if err != nil {
		return nil, err
	}
	return &currentTeamInfo, err
}

func getPrivateSpaces(ctx context.Context, teamUserID string, srv *rest.Client) ([]workspaceInfo, error) {
	var privateSpaceListResponse api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/" + teamUserID + "/privatespace",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &privateSpaceListResponse)
	if err != nil {
		return nil, err
	}

	workspaceList := make([]workspaceInfo, 0, len(privateSpaceListResponse.TeamWorkspace))
	for _, workspace := range privateSpaceListResponse.TeamWorkspace {
		workspaceList = append(workspaceList, workspaceInfo{ID: workspace.ID, Name: "My Space"})
	}
	return workspaceList, err
}

func listTeams(ctx context.Context, zuid string, srv *rest.Client) ([]api.TeamWorkspace, error) {
	var teamList api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/users/" + zuid + "/teams",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &teamList)
	if err != nil {
		return nil, err
	}
	return teamList.TeamWorkspace, nil
}

func listWorkspaces(ctx context.Context, teamID string, srv *rest.Client) ([]workspaceInfo, error) {
	var workspaceListResponse api.TeamWorkspaceResponse
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/teams/" + teamID + "/workspaces",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	_, err := srv.CallJSON(ctx, &opts, nil, &workspaceListResponse)
	if err != nil {
		return nil, err
	}

	workspaceList := make([]workspaceInfo, 0, len(workspaceListResponse.TeamWorkspace))
	for _, workspace := range workspaceListResponse.TeamWorkspace {
		workspaceList = append(workspaceList, workspaceInfo{ID: workspace.ID, Name: workspace.Attributes.Name})
	}

	return workspaceList, nil
}

// --------------------------------------------------------------

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// logThrottle logs the first 429 in a throttling episode at NOTICE level.
//
// The err value contains the server response body. Further 429s are not logged
// until shouldRetry observes a real recovery, which prevents sustained
// throttling from flooding the log. The pacer still logs every retry at DEBUG.
// Using recovery instead of a fixed time window works with both Zoho penalty
// regimes.
func (f *Fs) logThrottle(wait time.Duration, err error) {
	newBurst := f.throttle.progress.Swap(false)
	f.throttle.penaltyUntilNano.Store(time.Now().Add(wait).UnixNano())
	secs := int(wait / time.Second)
	if newBurst {
		fs.Logf(f, "Too many requests: Trying again in %d seconds. %v", secs, err)
	}
}

// isMissingResourceErr reports whether resp/err is Zoho's 401 "R008 Unauthorized
// access" - returned (not a 404) for a resource id (folder or file) that was
// deleted or never existed. A freshly refreshed token still gets it, so it is a
// missing resource, not a token problem.
func isMissingResourceErr(resp *http.Response, err error) bool {
	return resp != nil && resp.StatusCode == 401 && err != nil && strings.Contains(err.Error(), "R008")
}

// shouldRetry reports whether the given resp and err deserve to be retried.
//
// A 429 is honoured via the Retry-After header (falling back to 60s plus a
// margin) and starts or continues a throttling episode; expired OAuth tokens
// are retried, missing OAuth scopes abort, and standard HTTP retry conditions
// are also handled. A missing folder (R008) is not retried.
//
// Returns whether to retry, and the err as a convenience.
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	if err == nil && resp != nil && resp.StatusCode < 400 {
		// Treat as recovered only after the latest 429 wait ends, so in-flight
		// successes don't start a new throttling episode too early.
		if time.Now().UnixNano() > f.throttle.penaltyUntilNano.Load() {
			f.throttle.progress.Store(true)
		}
	}
	authRetry := false

	// Bail out early if we are missing OAuth Scopes.
	if resp != nil && resp.StatusCode == 401 && strings.Contains(resp.Status, "INVALID_OAUTHSCOPE") {
		fs.Errorf(nil, "zoho: missing OAuth Scope. Run rclone config reconnect to fix this issue.")
		return false, err
	}

	if resp != nil && resp.StatusCode == 401 && len(resp.Header["Www-Authenticate"]) == 1 && strings.Contains(resp.Header["Www-Authenticate"][0], "expired_token") {
		authRetry = true
		fs.Debugf(nil, "Should retry: %v", err)
	}

	// A missing resource never reappears, and retrying escalates to a 429 F7008.
	if isMissingResourceErr(resp, err) {
		return false, err
	}
	if resp != nil && resp.StatusCode == 429 {
		// Zoho's listing API is heavily rate limited and tells us how long to
		// wait in the Retry-After header. Honour it so we don't retry too early
		// (which makes Zoho escalate the penalty), falling back to 60s.
		if values := resp.Header["Retry-After"]; len(values) == 1 && values[0] != "" {
			retryAfter, parseErr := strconv.Atoi(values[0])
			if parseErr != nil {
				fs.Logf(f, "Failed to parse Retry-After: %q: %v", values[0], parseErr)
			} else {
				wait := time.Duration(retryAfter)*time.Second + retryAfterMargin
				f.logThrottle(wait, err)
				return true, pacer.RetryAfterError(err, wait)
			}
		}
		wait := 60*time.Second + retryAfterMargin
		f.logThrottle(wait, err)
		return true, pacer.RetryAfterError(err, wait)
	}
	return authRetry || fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// --------------------------------------------------------------

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
	return fmt.Sprintf("zoho root '%s'", f.root)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return fs.ModTimeNotSupported
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// parsePath parses a zoho 'url'
func parsePath(path string) (root string) {
	root = strings.Trim(path, "/")
	return
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.Item, err error) {
	// defer log.Trace(f, "path=%q", path)("info=%+v, err=%v", &info, &err)
	leaf, directoryID, err := f.dirCache.FindPath(ctx, path, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	found, err := f.listAll(ctx, directoryID, false, true, func(item *api.Item) bool {
		if item.Attributes.Name == leaf {
			info = item
			return true
		}
		return false
	})
	if err != nil {
		if err == fs.ErrorDirNotFound {
			// The cached parent directory id is stale: its folder was deleted, so
			// listing it returned R008 (mapped to ErrorDirNotFound). Flush the stale
			// entry so a later create re-resolves (and recreates) the parent, and
			// report the object as not found - it cannot exist if its parent is gone.
			parent := strings.TrimSuffix(path[:len(path)-len(leaf)], "/")
			fs.Debugf(f, "readMetaDataForPath %q: parent %q stale (R008), flushing dircache", path, parent)
			f.dirCache.FlushDir(parent)
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}
	if !found {
		return nil, fs.ErrorObjectNotFound
	}
	return info, nil
}

// readMetaDataForID reads the metadata for the object with given ID
func (f *Fs) readMetaDataForID(ctx context.Context, id string) (*api.Item, error) {
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/files/" + id,
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
		Parameters:   url.Values{},
	}
	var result *api.ItemInfo
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return &result.Item, nil
}

// tpsMinSleep converts a transactions-per-second rate into the pacer's minimum
// sleep between calls.
func tpsMinSleep(tps float64) time.Duration {
	return time.Duration(float64(time.Second) / tps)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	if opt.UploadCutoff < defaultUploadCutoff {
		return nil, fmt.Errorf("zoho: upload cutoff (%v) must be greater than equal to %v", opt.UploadCutoff, fs.SizeSuffix(defaultUploadCutoff))
	}

	err := setupRegion(m)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)
	oAuthClient, _, err := oauthutil.NewClient(ctx, name, m, oauthConfig)
	if err != nil {
		return nil, err
	}

	// Add a delay between API calls to respect Zoho's per-second request limit.
	// The wait time is calculated from the configured TPS value and retried on errors.
	pacerMinSleep := minSleep
	pacerBurst := 1
	if opt.TPSLimit > 0 {
		pacerMinSleep = tpsMinSleep(opt.TPSLimit)
		pacerBurst = max(opt.TPSLimitBurst, 1)
	}

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		srv:         rest.NewClient(oAuthClient).SetRoot(rootURL),
		downloadsrv: rest.NewClient(oAuthClient).SetRoot(downloadURL),
		uploadsrv:   rest.NewClient(oAuthClient).SetRoot(uploadURL),
		pacer:       fs.NewPacer(ctx, pacer.NewGoogleDrive(pacer.MinSleep(pacerMinSleep), pacer.Burst(pacerBurst))),
		throttle:    &throttleState{},
	}
	// Arm progress so the very first 429 is logged at NOTICE.
	f.throttle.progress.Store(true)
	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)

	// Get rootFolderID
	rootID := f.opt.RootFolderID
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.newObjectWithInfo(ctx, remote, nil)
		if err != nil {
			if err == fs.ErrorObjectNotFound {
				// File doesn't exist so return old f
				return f, nil
			}
			return nil, err
		}
		f.features.Fill(ctx, &tempF)
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(*api.Item) bool

// folderListLimiter provides the shared per-folder listing rate limiter for dirID.
//
// Zoho WorkDrive throttles its listing API (GET files/{id}/files) PER folder,
// independently of --zoho-tpslimit. Measured live (2026-07-05), that per-folder
// throttle is a sliding window: ~19 listings of one folder within any rolling
// ~60s are allowed and the 20th returns F7008 with a ~300s Retry-After. Every
// observed trip landed exactly on the 20th listing inside 60s and every clean
// run stayed at <=19, including a deliberate over-limit probe (window total 22)
// tripping at #20; an earlier token-bucket refill model fitted the same data
// inconsistently. There is also a much smaller instantaneous cap (~15-16
// back-to-back listings trip on their own), so burst must stay well below that.
//
// The limiter (folderWindowLimiter) guarantees at most limit listings per
// window for any traffic pattern: each window starts with burst listings
// passing back-to-back - the burst RE-ARMS at every window boundary - and the
// remaining limit-burst are paced window/(limit-burst) apart, while the
// sliding log of the last limit grants enforces the cap across window
// boundaries and idle resumes. Raising burst therefore never raises the
// per-window total - it only front-loads it, widening the sustained spacing
// to compensate. Different folders, or a folder listed fewer than burst
// times, are never delayed - only tps applies unless the SAME folder is
// re-listed enough to drain the burst.
//
// It paces repeated listings of the same folder; without it, listing one folder
// too often trips Zoho's per-folder throttle and returns HTTP 429. The limiter is
// created on demand in folderListLimiterRegistry, and limiters idle longer than
// the window are evicted (at most once per window): after a full window idle
// Zoho's sliding window is empty and the limiter is fully refilled, so dropping
// it is lossless. Only called when --zoho-list-folder-limit > 0, so limit is
// never zero.
//
// Returns the folder's *folderWindowLimiter, ready to Wait on before listing.
func (f *Fs) folderListLimiter(dirID string) *folderWindowLimiter {
	flReg := folderListLimiterRegistry
	window := time.Duration(f.opt.ListFolderWindow)
	// Key by region+folder-id so one limiter is shared per physical folder
	// across every *Fs without colliding across accounts or regions.
	key := f.opt.Region + "\x00" + dirID
	flReg.mu.Lock()
	defer flReg.mu.Unlock()
	now := time.Now()
	if now.Sub(flReg.lastSweep) > window || len(flReg.limiters) > folderListLimitersMaxEntries {
		before := len(flReg.limiters)
		for id, e := range flReg.limiters {
			if now.Sub(e.lastListed) > window {
				delete(flReg.limiters, id)
			}
		}
		flReg.lastSweep = now
		fs.Debugf(f, "folder listing limiter sweep: evicted %d of %d idle limiters", before-len(flReg.limiters), before)
	}
	e := flReg.limiters[key]
	if e == nil {
		// The first Fs to list this folder fixes the shape; in the normal case
		// every Fs for the same account shares the same limit/window/burst.
		// limit > 0 is guaranteed by the caller; clamp burst into [1, limit-1]
		// (at limit 1 the burst token itself is the whole window's budget).
		burst := min(max(f.opt.ListFolderBurst, 1), max(f.opt.ListFolderLimit-1, 1))
		intervalTokens := max(f.opt.ListFolderLimit-burst, 1)
		e = &folderListLimiterEntry{limiter: &folderWindowLimiter{
			window:   window,
			limit:    f.opt.ListFolderLimit,
			burst:    burst,
			interval: window / time.Duration(intervalTokens),
		}}
		flReg.limiters[key] = e
	}
	e.lastListed = now
	return e.limiter
}

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dirID string, directoriesOnly bool, filesOnly bool, fn listAllFn) (found bool, err error) {
	// Pace repeated listings of the SAME folder under Zoho's per-folder throttle
	// (see --zoho-list-folder-limit). One token per listAll call, not per page,
	// so multi-page listings of a single folder are not penalised.
	if f.opt.ListFolderLimit > 0 {
		if err = f.folderListLimiter(dirID).Wait(ctx); err != nil {
			return false, err
		}
	}
	const listItemsLimit = 1000
	opts := rest.Opts{
		Method:       "GET",
		Path:         "/files/" + dirID + "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
		Parameters: url.Values{
			"page[limit]": {strconv.Itoa(listItemsLimit)},
			"page[next]":  {"0"},
		},
	}
OUTER:
	for {
		var result api.ItemList
		var resp *http.Response
		err = f.pacer.Call(func() (bool, error) {
			resp, err = f.srv.CallJSON(ctx, &opts, nil, &result)
			return f.shouldRetry(ctx, resp, err)
		})
		if err != nil {
			// Surface a missing folder as directory-not-found so dircache/the VFS
			// treat it as gone instead of hard-failing on a stale id.
			if isMissingResourceErr(resp, err) {
				fs.Debugf(f, "listAll %q: R008 unauthorized - treating as directory not found", dirID)
				return false, fs.ErrorDirNotFound
			}
			return false, fmt.Errorf("couldn't list files: %w", err)
		}
		if len(result.Items) == 0 {
			break
		}
		for i := range result.Items {
			item := &result.Items[i]
			if item.Attributes.IsFolder {
				if filesOnly {
					continue
				}
			} else {
				if directoriesOnly {
					continue
				}
			}
			item.Attributes.Name = f.opt.Enc.ToStandardName(item.Attributes.Name)
			if fn(item) {
				found = true
				break OUTER
			}
		}
		if !result.Links.Cursor.HasNext {
			break
		}
		// Fetch the next from the URL in the response
		nextURL, err := url.Parse(result.Links.Cursor.Next)
		if err != nil {
			return found, fmt.Errorf("failed to parse next link as URL: %w", err)
		}
		opts.Parameters.Set("page[next]", nextURL.Query().Get("page[next]"))
	}
	return
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}
	var iErr error
	_, err = f.listAll(ctx, directoryID, false, false, func(info *api.Item) bool {
		remote := path.Join(dir, info.Attributes.Name)
		if info.Attributes.IsFolder {
			// cache the directory ID for later lookups
			f.dirCache.Put(remote, info.ID)
			d := fs.NewDir(remote, time.Time(info.Attributes.ModifiedTime)).SetID(info.ID)
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (pathIDOut string, found bool, err error) {
	// Find the leaf in pathID
	found, err = f.listAll(ctx, pathID, true, false, func(item *api.Item) bool {
		if item.Attributes.Name == leaf {
			pathIDOut = item.ID
			return true
		}
		return false
	})
	return pathIDOut, found, err
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (newID string, err error) {
	//fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	var info *api.ItemInfo
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	mkdir := api.WriteMetadataRequest{
		Data: api.WriteMetadata{
			Attributes: api.WriteAttributes{
				Name:     f.opt.Enc.FromStandardName(leaf),
				ParentID: pathID,
			},
			Type: "files",
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &mkdir, &info)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return "", err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return info.Item.ID, nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Item) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createObject(ctx context.Context, remote string, size int64, modTime time.Time) (o *Object, leaf string, directoryID string, err error) {
	leaf, directoryID, err = f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return
	}
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: modTime,
	}
	return
}

func (f *Fs) uploadLargeFile(ctx context.Context, name string, parent string, size int64, in io.Reader, options ...fs.OpenOption) (*api.Item, error) {
	opts := rest.Opts{
		Method:        "POST",
		Path:          "/stream/upload",
		Body:          in,
		ContentLength: &size,
		ContentType:   "application/octet-stream",
		Options:       options,
		ExtraHeaders: map[string]string{
			"x-filename":  url.QueryEscape(name),
			"x-parent_id": parent,
			// Must carry the x- prefix; without it the stream endpoint ignores
			// the flag and creates a duplicate instead of overwriting.
			"x-override-name-exist": "true",
			"upload-id":             uuid.New().String(),
			"x-streammode":          "1",
		},
	}

	var err error
	var resp *http.Response
	var uploadResponse *api.LargeUploadResponse
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.uploadsrv.CallJSON(ctx, &opts, nil, &uploadResponse)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("upload large error: %v", err)
	}
	if len(uploadResponse.Uploads) != 1 {
		return nil, errors.New("upload: invalid response")
	}
	upload := uploadResponse.Uploads[0]
	uploadInfo, err := upload.GetUploadFileInfo()
	if err != nil {
		return nil, fmt.Errorf("upload error: %w", err)
	}

	// Fill in the api.Item from the api.UploadFileInfo
	var info api.Item
	info.ID = upload.Attributes.RessourceID
	info.Attributes.Name = upload.Attributes.FileName
	// info.Attributes.Type = not used
	info.Attributes.IsFolder = false
	// info.Attributes.CreatedTime = not used
	info.Attributes.ModifiedTime = uploadInfo.GetModTime()
	// info.Attributes.UploadedTime = 0 not used
	info.Attributes.StorageInfo.Size = uploadInfo.Size
	info.Attributes.StorageInfo.FileCount = 0
	info.Attributes.StorageInfo.FolderCount = 0

	return &info, nil
}

func (f *Fs) upload(ctx context.Context, name string, parent string, size int64, in io.Reader, options ...fs.OpenOption) (*api.Item, error) {
	params := url.Values{}
	params.Set("filename", url.QueryEscape(name))
	params.Set("parent_id", parent)
	params.Set("override-name-exist", strconv.FormatBool(true))
	formReader, contentType, overhead, err := rest.MultipartUpload(ctx, in, nil, "content", name, "application/octet-stream")
	if err != nil {
		return nil, fmt.Errorf("failed to make multipart upload: %w", err)
	}

	contentLength := overhead + size
	opts := rest.Opts{
		Method:           "POST",
		Path:             "/upload",
		Body:             formReader,
		ContentType:      contentType,
		ContentLength:    &contentLength,
		Options:          options,
		Parameters:       params,
		TransferEncoding: []string{"identity"},
	}

	var resp *http.Response
	var uploadResponse *api.UploadResponse
	err = f.pacer.CallNoRetry(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, nil, &uploadResponse)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("upload error: %w", err)
	}
	if len(uploadResponse.Uploads) != 1 {
		return nil, errors.New("upload: invalid response")
	}
	upload := uploadResponse.Uploads[0]
	uploadInfo, err := upload.GetUploadFileInfo()
	if err != nil {
		return nil, fmt.Errorf("upload error: %w", err)
	}

	// Fill in the api.Item from the api.UploadFileInfo
	var info api.Item
	info.ID = upload.Attributes.RessourceID
	info.Attributes.Name = upload.Attributes.FileName
	// info.Attributes.Type = not used
	info.Attributes.IsFolder = false
	// info.Attributes.CreatedTime = not used
	info.Attributes.ModifiedTime = uploadInfo.GetModTime()
	// info.Attributes.UploadedTime = 0 not used
	info.Attributes.StorageInfo.Size = uploadInfo.Size
	info.Attributes.StorageInfo.FileCount = 0
	info.Attributes.StorageInfo.FolderCount = 0

	return &info, nil
}

// Put the object into the container
//
// Copy the reader in to the new object which is returned.
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		size := src.Size()
		remote := src.Remote()

		// Create the directory for the object if it doesn't exist
		leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
		if err != nil {
			return nil, err
		}

		// use normal upload API for small sizes (<10MiB)
		if size < int64(f.opt.UploadCutoff) {
			info, err := f.upload(ctx, f.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
			if err != nil {
				return nil, err
			}

			return f.newObjectWithInfo(ctx, remote, info)
		}

		// large file API otherwise
		info, err := f.uploadLargeFile(ctx, f.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return nil, err
		}

		return f.newObjectWithInfo(ctx, remote, info)
	default:
		return nil, err
	}
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// deleteObject removes an object by ID
func (f *Fs) deleteObject(ctx context.Context, id string) (err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	delete := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					Status: "51", // Status "51" is deleted
				},
				ID:   id,
				Type: "files",
			},
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &delete, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("delete object failed: %w", err)
	}
	return nil
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}
	rootID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	info, err := f.readMetaDataForID(ctx, rootID)
	if err != nil {
		return err
	}
	if check && info.Attributes.StorageInfo.Size > 0 {
		return fs.ErrorDirectoryNotEmpty
	}

	err = f.deleteObject(ctx, rootID)
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
	}
	f.dirCache.FlushDir(dir)
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Purge deletes all the files and the container
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

func (f *Fs) rename(ctx context.Context, id, name string) (item *api.Item, err error) {
	var resp *http.Response
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files/" + id,
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	rename := api.WriteMetadataRequest{
		Data: api.WriteMetadata{
			Attributes: api.WriteAttributes{
				Name: f.opt.Enc.FromStandardName(name),
			},
			Type: "files",
		},
	}
	var result *api.ItemInfo
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &rename, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("rename failed: %w", err)
	}
	return &result.Item, nil
}

// Copy src to this remote using server side copy operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantCopy
	}
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObject, leaf, directoryID, err := f.createObject(ctx, remote, srcObj.size, srcObj.modTime)
	if err != nil {
		return nil, err
	}
	// Copy the object
	opts := rest.Opts{
		Method:       "POST",
		Path:         "/files/" + directoryID + "/copy",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	copyFile := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					RessourceID: srcObj.id,
				},
				Type: "files",
			},
		},
	}
	var resp *http.Response
	var result *api.ItemList
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &copyFile, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("couldn't copy file: %w", err)
	}
	// Server acts weird some times make sure we actually got
	// an item
	if len(result.Items) != 1 {
		return nil, errors.New("couldn't copy file: invalid response")
	}
	// Only set ID here because response is not complete Item struct
	dstObject.id = result.Items[0].ID

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if f.opt.Enc.ToStandardName(result.Items[0].Attributes.Name) != leaf {
		if err = dstObject.rename(ctx, leaf); err != nil {
			return nil, fmt.Errorf("copy: couldn't rename copied file: %w", err)
		}
	}
	return dstObject, nil
}

func (f *Fs) move(ctx context.Context, srcID, parentID string) (item *api.Item, err error) {
	// Move the object
	opts := rest.Opts{
		Method:       "PATCH",
		Path:         "/files",
		ExtraHeaders: map[string]string{"Accept": "application/vnd.api+json"},
	}
	moveFile := api.WriteMultiMetadataRequest{
		Meta: []api.WriteMetadata{
			{
				Attributes: api.WriteAttributes{
					ParentID: parentID,
				},
				ID:   srcID,
				Type: "files",
			},
		},
	}
	var resp *http.Response
	var result *api.ItemList
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallJSON(ctx, &opts, &moveFile, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("move failed: %w", err)
	}
	// Server acts weird some times make sure our array actually contains
	// a file
	if len(result.Items) != 1 {
		return nil, errors.New("move failed: invalid response")
	}
	return &result.Items[0], nil
}

// Move src to this remote using server side move operations.
//
// This is stored with the remote path given.
//
// It returns the destination Object and a possible error.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}
	err := srcObj.readMetaData(ctx)
	if err != nil {
		return nil, err
	}

	srcLeaf, srcParentID, err := srcObj.fs.dirCache.FindPath(ctx, srcObj.remote, false)
	if err != nil {
		return nil, err
	}

	// Create temporary object
	dstObject, dstLeaf, directoryID, err := f.createObject(ctx, remote, srcObj.size, srcObj.modTime)
	if err != nil {
		return nil, err
	}

	needRename := srcLeaf != dstLeaf
	needMove := srcParentID != directoryID

	// rename the leaf to a temporary name if we are moving to
	// another directory to make sure we don't overwrite something
	// in the source directory by accident
	if needRename && needMove {
		tmpLeaf := "rcloneTemp" + random.String(8)
		if err = srcObj.rename(ctx, tmpLeaf); err != nil {
			return nil, fmt.Errorf("move: pre move rename failed: %w", err)
		}
	}

	// do the move if required
	if needMove {
		item, err := f.move(ctx, srcObj.id, directoryID)
		if err != nil {
			return nil, err
		}
		// Only set ID here because response is not complete Item struct
		dstObject.id = item.ID
	} else {
		// same parent only need to rename
		dstObject.id = srcObj.id
	}

	// rename the leaf to its final name
	if needRename {
		if err = dstObject.rename(ctx, dstLeaf); err != nil {
			return nil, fmt.Errorf("move: couldn't rename moved file: %w", err)
		}
	}
	return dstObject, nil
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, srcDirectoryID, srcLeaf, dstDirectoryID, dstLeaf, err := f.dirCache.DirMove(ctx, srcFs.dirCache, srcFs.root, srcRemote, f.root, dstRemote)
	if err != nil {
		return err
	}
	// same parent only need to rename
	if srcDirectoryID == dstDirectoryID {
		_, err = f.rename(ctx, srcID, dstLeaf)
		return err
	}

	// do the move
	_, err = f.move(ctx, srcID, dstDirectoryID)
	if err != nil {
		return fmt.Errorf("couldn't dir move: %w", err)
	}

	// Can't copy and change name in one step so we have to check if we have
	// the correct name after copy
	if srcLeaf != dstLeaf {
		_, err = f.rename(ctx, srcID, dstLeaf)
		if err != nil {
			return fmt.Errorf("dirmove: couldn't rename moved dir: %w", err)
		}
	}
	srcFs.dirCache.FlushDir(srcRemote)
	return nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (usage *fs.Usage, err error) {
	info, err := f.readMetaDataForID(ctx, f.opt.RootFolderID)
	if err != nil {
		return nil, err
	}
	usage = &fs.Usage{
		Used: fs.NewUsageValue(info.Attributes.StorageInfo.Size),
	}
	return usage, nil
}

// DirCacheFlush resets the directory cache - used in testing as an
// optional interface
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
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

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Hash returns the SHA-1 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	if err := o.readMetaData(context.TODO()); err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Item) (err error) {
	if info.Attributes.IsFolder {
		return fs.ErrorIsDir
	}
	o.hasMetaData = true
	o.size = info.Attributes.StorageInfo.Size
	o.modTime = time.Time(info.Attributes.ModifiedTime)
	o.id = info.ID
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// rename renames an object in place
//
// this a separate api call then move with zoho
func (o *Object) rename(ctx context.Context, name string) (err error) {
	item, err := o.fs.rename(ctx, o.id, name)
	if err != nil {
		return err
	}
	return o.setMetaData(item)
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	if o.id == "" {
		return nil, errors.New("can't download - no id")
	}
	var resp *http.Response
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		Path:    "/download/" + o.id,
		Options: options,
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.downloadsrv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one.
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	size := src.Size()
	remote := o.Remote()

	// Create the directory for the object if it doesn't exist
	leaf, directoryID, err := o.fs.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return err
	}

	// use normal upload API for small sizes (<10MiB)
	if size < int64(o.fs.opt.UploadCutoff) {
		info, err := o.fs.upload(ctx, o.fs.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
		if err != nil {
			return err
		}

		return o.setMetaData(info)
	}

	// large file API otherwise
	info, err := o.fs.uploadLargeFile(ctx, o.fs.opt.Enc.FromStandardName(leaf), directoryID, size, in, options...)
	if err != nil {
		return err
	}

	return o.setMetaData(info)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	return o.fs.deleteObject(ctx, o.id)
}

// ID returns the ID of the Object if known, or "" if not
func (o *Object) ID() string {
	return o.id
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.Purger          = (*Fs)(nil)
	_ fs.Copier          = (*Fs)(nil)
	_ fs.Abouter         = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.IDer            = (*Object)(nil)
)
