// Package rc implements a remote control server and registry for rclone
//
// To register your internal calls, call rc.Add(path, function).  Your
// function should take ane return a Param.  It can also return an
// error.  Use rc.NewError to wrap an existing error along with an
// http response type if another response other than 500 internal
// error is required on error.
package rc

import (
	"encoding/json"
	"io"
	_ "net/http/pprof" // install the pprof http handlers

	"github.com/rclone/rclone/cmd/serve/httplib"
)

// Options contains options for the remote control server
type Options struct {
	HTTPOptions    httplib.Options
	Enabled        bool   // set to enable the server
	Serve          bool   // set to serve files from remotes
	Files          string // set to enable serving files locally
	NoAuth         bool   // set to disable auth checks on AuthRequired methods
	WebUI          bool   // set to launch the web ui
	WebGUIUpdate   bool   // set to download new update
	WebGUIFetchURL string // set the default url for fetching webgui

}

// DefaultOpt is the default values used for Options
var DefaultOpt = Options{
	HTTPOptions: httplib.DefaultOpt,
	Enabled:     false,
}

func init() {
	DefaultOpt.HTTPOptions.ListenAddr = "localhost:5572"
}

// WriteJSON writes JSON in out to w
func WriteJSON(w io.Writer, out Params) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "\t")
	return enc.Encode(out)
}
