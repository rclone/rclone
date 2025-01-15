// Package rc implements a remote control server and registry for rclone
//
// To register your internal calls, call rc.Add(path, function).  Your
// function should take and return a Param.  It can also return an
// error.  Use rc.NewError to wrap an existing error along with an
// http response type if another response other than 500 internal
// error is required on error.
package rc

import (
	"encoding/json"
	"io"
	_ "net/http/pprof" // install the pprof http handlers
	"time"

	"github.com/rclone/rclone/fs"
	libhttp "github.com/rclone/rclone/lib/http"
)

// OptionsInfo describes the Options in use
var OptionsInfo = fs.Options{{
	Name:    "rc",
	Default: false,
	Help:    "Enable the remote control server",
	Groups:  "RC",
}, {
	Name:    "rc_files",
	Default: "",
	Help:    "Path to local files to serve on the HTTP server",
	Groups:  "RC",
}, {
	Name:    "rc_serve",
	Default: false,
	Help:    "Enable the serving of remote objects",
	Groups:  "RC",
}, {
	Name:    "rc_serve_no_modtime",
	Default: false,
	Help:    "Don't read the modification time (can speed things up)",
	Groups:  "RC",
}, {
	Name:    "rc_no_auth",
	Default: false,
	Help:    "Don't require auth for certain methods",
	Groups:  "RC",
}, {
	Name:    "rc_web_gui",
	Default: false,
	Help:    "Launch WebGUI on localhost",
	Groups:  "RC",
}, {
	Name:    "rc_web_gui_update",
	Default: false,
	Help:    "Check and update to latest version of web gui",
	Groups:  "RC",
}, {
	Name:    "rc_web_gui_force_update",
	Default: false,
	Help:    "Force update to latest version of web gui",
	Groups:  "RC",
}, {
	Name:    "rc_web_gui_no_open_browser",
	Default: false,
	Help:    "Don't open the browser automatically",
	Groups:  "RC",
}, {
	Name:    "rc_web_fetch_url",
	Default: "https://api.github.com/repos/rclone/rclone-webui-react/releases/latest",
	Help:    "URL to fetch the releases for webgui",
	Groups:  "RC",
}, {
	Name:    "rc_enable_metrics",
	Default: false,
	Help:    "Enable the Prometheus metrics path at the remote control server",
	Groups:  "RC,Metrics",
}, {
	Name:    "rc_job_expire_duration",
	Default: 60 * time.Second,
	Help:    "Expire finished async jobs older than this value",
	Groups:  "RC",
}, {
	Name:    "rc_job_expire_interval",
	Default: 10 * time.Second,
	Help:    "Interval to check for expired async jobs",
	Groups:  "RC",
}, {
	Name:    "metrics_addr",
	Default: []string{},
	Help:    "IPaddress:Port or :Port to bind metrics server to",
	Groups:  "Metrics",
}}.
	AddPrefix(libhttp.ConfigInfo, "rc", "RC").
	AddPrefix(libhttp.AuthConfigInfo, "rc", "RC").
	AddPrefix(libhttp.TemplateConfigInfo, "rc", "RC").
	AddPrefix(libhttp.ConfigInfo, "metrics", "Metrics").
	AddPrefix(libhttp.AuthConfigInfo, "metrics", "Metrics").
	AddPrefix(libhttp.TemplateConfigInfo, "metrics", "Metrics").
	SetDefault("rc_addr", []string{"localhost:5572"})

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "rc", Opt: &Opt, Options: OptionsInfo})
}

// Options contains options for the remote control server
type Options struct {
	HTTP                libhttp.Config         `config:"rc"`
	Auth                libhttp.AuthConfig     `config:"rc"`
	Template            libhttp.TemplateConfig `config:"rc"`
	Enabled             bool                   `config:"rc"`                         // set to enable the server
	Files               string                 `config:"rc_files"`                   // set to enable serving files locally
	Serve               bool                   `config:"rc_serve"`                   // set to serve files from remotes
	ServeNoModTime      bool                   `config:"rc_serve_no_modtime"`        // don't read the modification time
	NoAuth              bool                   `config:"rc_no_auth"`                 // set to disable auth checks on AuthRequired methods
	WebUI               bool                   `config:"rc_web_gui"`                 // set to launch the web ui
	WebGUIUpdate        bool                   `config:"rc_web_gui_update"`          // set to check new update
	WebGUIForceUpdate   bool                   `config:"rc_web_gui_force_update"`    // set to force download new update
	WebGUINoOpenBrowser bool                   `config:"rc_web_gui_no_open_browser"` // set to disable auto opening browser
	WebGUIFetchURL      string                 `config:"rc_web_fetch_url"`           // set the default url for fetching webgui
	EnableMetrics       bool                   `config:"rc_enable_metrics"`          // set to disable prometheus metrics on /metrics
	MetricsHTTP         libhttp.Config         `config:"metrics"`
	MetricsAuth         libhttp.AuthConfig     `config:"metrics"`
	MetricsTemplate     libhttp.TemplateConfig `config:"metrics"`
	JobExpireDuration   time.Duration          `config:"rc_job_expire_duration"`
	JobExpireInterval   time.Duration          `config:"rc_job_expire_interval"`
}

// Opt is the default values used for Options
var Opt Options

// WriteJSON writes JSON in out to w
func WriteJSON(w io.Writer, out Params) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return enc.Encode(out)
}
