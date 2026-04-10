// Package gui implements the "rclone gui" command.
package gui

import (
	"context"
	"embed"
	"flag"
	"fmt"
	iofs "io/fs"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/rclone/rclone/cmd"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fs/rc/rcserver"
	libhttp "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/lib/systemd"
	"github.com/skratchdot/open-golang/open"
	"github.com/spf13/cobra"
)

//go:embed dist
var assets embed.FS

var (
	guiAddr       []string
	apiAddr       []string
	user          string
	pass          string
	noAuth        bool
	noOpenBrowser bool
	enableMetrics bool
)

func init() {
	cmd.Root.AddCommand(commandDefinition)
	f := commandDefinition.Flags()
	f.StringArrayVar(&guiAddr, "addr", nil, "IPaddress:Port for the GUI server (default auto-chosen localhost port)")
	f.StringArrayVar(&apiAddr, "api-addr", nil, "IPaddress:Port for the RC API server (default auto-chosen localhost port)")
	f.StringVar(&user, "user", "", "User name for RC authentication")
	f.StringVar(&pass, "pass", "", "Password for RC authentication")
	f.BoolVar(&noAuth, "no-auth", false, "Don't require auth for the RC API")
	f.BoolVar(&noOpenBrowser, "no-open-browser", false, "Skip opening the browser automatically")
	f.BoolVar(&enableMetrics, "enable-metrics", false, "Enable OpenMetrics/Prometheus compatible endpoint at /metrics")
}

var commandDefinition = &cobra.Command{
	Use:   "gui",
	Short: `Open the web based GUI.`,
	Long: `This command starts an embedded web GUI for rclone and opens it in
your default browser.

It starts an RC API server and a GUI server on separate localhost
ports, generates login credentials automatically unless --no-auth
is specified, and opens the browser already authenticated.

    rclone gui

Use --no-open-browser to skip opening the browser automatically:

    rclone gui --no-open-browser

Use --addr to bind the GUI to a specific address:

    rclone gui --addr localhost:5580

Use --user and --pass to set specific credentials:

    rclone gui --user admin --pass secret

Use --no-auth to disable authentication entirely:

    rclone gui --no-auth
`,
	Annotations: map[string]string{
		"versionIntroduced": "v1.74",
		"groups":            "RC",
	},
	Run: func(command *cobra.Command, args []string) {
		cmd.CheckArgs(0, 0, command, args)
		ctx := context.Background()

		// --- 1. Create the GUI server (binds port eagerly, before Serve) ---
		guiCfg := libhttp.DefaultCfg()
		if command.Flags().Changed("addr") {
			guiCfg.ListenAddr = guiAddr
		} else {
			guiCfg.ListenAddr = []string{"localhost:0"}
		}
		guiServer, err := libhttp.NewServer(ctx, libhttp.WithConfig(guiCfg))
		if err != nil {
			fs.Fatalf(nil, "Failed to create GUI server: %v", err)
		}

		// Read the GUI origin from the bound address (available before Serve).
		guiOrigin := originFromURL(guiServer.URLs()[0])

		// --- 2. Configure the RC API server ---
		opt := rc.Opt // copy global defaults
		opt.Enabled = true
		opt.WebUI = false
		opt.Serve = false

		if command.Flags().Changed("api-addr") {
			opt.HTTP.ListenAddr = apiAddr
		} else {
			port, err := freePort()
			if err != nil {
				fs.Fatalf(nil, "Failed to find a free port for RC: %v", err)
			}
			opt.HTTP.ListenAddr = []string{fmt.Sprintf("localhost:%d", port)}
		}

		// CORS: allow the GUI origin to make cross-port API requests.
		opt.HTTP.AllowOrigin = guiOrigin

		// Forward metrics flag to the RC server.
		if command.Flags().Changed("enable-metrics") {
			opt.EnableMetrics = enableMetrics
		}

		// --- 3. Generate credentials if needed ---
		if command.Flags().Changed("user") {
			opt.Auth.BasicUser = user
		}
		if command.Flags().Changed("pass") {
			opt.Auth.BasicPass = pass
		}
		if command.Flags().Changed("no-auth") {
			opt.NoAuth = noAuth
		}

		if !opt.NoAuth {
			if opt.Auth.BasicUser == "" {
				opt.Auth.BasicUser = "gui"
				fs.Infof(nil, "No username specified. Using default username: %s", opt.Auth.BasicUser)
			}
			if opt.Auth.BasicPass == "" {
				randomPass, err := random.Password(128)
				if err != nil {
					fs.Fatalf(nil, "Failed to make password: %v", err)
				}
				opt.Auth.BasicPass = randomPass
				fs.Infof(nil, "No password specified. Using random password: %s", randomPass)
			}
		}

		// --- 4. Start the RC server (unchanged rcserver.Start) ---
		rcServer, err := rcserver.Start(ctx, &opt)
		if err != nil {
			fs.Fatalf(nil, "Failed to start RC server: %v", err)
		}
		if rcServer == nil {
			fs.Fatal(nil, "RC server not configured")
		}

		// Build the RC URL from the address we configured (rcserver.Server
		// does not expose URLs, and we know the address we passed in).
		rcURL := "http://" + opt.HTTP.ListenAddr[0] + "/"

		// --- 5. Mount the embedded GUI handler and start serving ---
		spaHandler := guiHandler()
		guiServer.Router().Get("/*", spaHandler.ServeHTTP)
		guiServer.Router().Head("/*", spaHandler.ServeHTTP)
		guiServer.Serve()

		guiURL := guiServer.URLs()[0]
		fs.Logf(nil, "Serving GUI on %s", guiURL)

		// --- 6. Open browser ---
		loginURL := buildLoginURL(guiURL, rcURL, opt.Auth.BasicUser, opt.Auth.BasicPass, opt.NoAuth)

		fs.Logf(nil, "GUI available at %s", loginURL)
		if flag.Lookup("test.v") == nil && !noOpenBrowser {
			if err := open.Start(loginURL); err != nil {
				fs.Errorf(nil, "Failed to open GUI in browser: %v", err)
			}
		}

		// --- 7. Wait for either server to exit, then shut both down ---
		defer systemd.Notify()()
		done := make(chan struct{}, 2)
		go func() { rcServer.Wait(); done <- struct{}{} }()
		go func() { guiServer.Wait(); done <- struct{}{} }()
		<-done
		_ = rcServer.Shutdown()
		_ = guiServer.Shutdown()
	},
}

// freePort asks the OS for a free TCP port on localhost.
func freePort() (int, error) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// originFromURL extracts the origin (scheme://host) from a URL string,
// stripping any path or trailing slash.
func originFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return strings.TrimRight(rawURL, "/")
	}
	return u.Scheme + "://" + u.Host
}

// guiHandler returns an http.Handler that serves the embedded GUI bundle
// with SPA fallback: paths that don't match a real file return index.html.
func guiHandler() http.Handler {
	sub, err := iofs.Sub(assets, "dist")
	if err != nil {
		panic("gui: embedded dist missing: " + err.Error())
	}
	fileServer := http.FileServer(http.FS(sub))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := iofs.Stat(sub, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for unknown paths so that
		// client-side routing (e.g. /login) works.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// guiBaseURL is the GUI server's URL. rcURL is the RC API server's URL.
// When auth is enabled it appends url, user, and pass as query
// parameters so the React app can discover the API endpoint and
// log in automatically.
func buildLoginURL(guiBaseURL, rcURL, user, pass string, noAuth bool) string {
	u, err := url.Parse(guiBaseURL)
	if err != nil {
		return guiBaseURL
	}
	if noAuth {
		return u.String()
	}
	u.Path = "/login"
	q := u.Query()
	q.Set("url", rcURL)
	q.Set("user", user)
	q.Set("pass", pass)
	u.RawQuery = q.Encode()
	return u.String()
}
