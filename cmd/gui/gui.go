// Package gui implements the "rclone gui" command.
package gui

import (
	"archive/zip"
	"context"
	"embed"
	"flag"
	"fmt"
	iofs "io/fs"
	"net"
	"net/http"
	"net/url"
	"os"
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
	Use:   "gui [path]",
	Short: `Open the web based GUI.`,
	Long: `This command starts an embedded web GUI for rclone and opens it in
your default browser.

This starts an RC API server and a GUI server on separate localhost
ports, generates login credentials automatically unless --no-auth
is specified, and opens the browser already authenticated.

    rclone gui

By default rclone gui serves the web GUI that was embedded into the
rclone binary at build time from https://github.com/rclone/rclone-web/
You can override this by passing a path to either an unpacked GUI
directory or a dist.zip archive (e.g. one downloaded from the
rclone-web releases page):

    rclone gui ./my-dist/
    rclone gui ./dist.zip

This is useful for iterating on the GUI locally without rebuilding
rclone, or for serving a different GUI release than the one embedded.

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
	RunE: func(command *cobra.Command, args []string) error {
		cmd.CheckArgs(0, 1, command, args)
		ctx := context.Background()

		// Resolve the GUI source (embedded, directory, or .zip)
		// before binding any sockets so errors surface immediately.
		var srcPath string
		if len(args) == 1 {
			srcPath = args[0]
		}
		srcFS, cleanupSrc, err := guiSourceFS(srcPath)
		if err != nil {
			return err
		}
		defer func() { _ = cleanupSrc() }()

		// Create the GUI server (binds port eagerly, before Serve)
		guiCfg := libhttp.DefaultCfg()
		if command.Flags().Changed("addr") {
			guiCfg.ListenAddr = guiAddr
		} else {
			guiCfg.ListenAddr = []string{"localhost:0"}
		}
		guiServer, err := libhttp.NewServer(ctx, libhttp.WithConfig(guiCfg))
		if err != nil {
			return fmt.Errorf("failed to create GUI server: %w", err)
		}

		// Read the GUI origin from the bound address (available before Serve).
		guiOrigin := originFromURL(guiServer.URLs()[0])

		// Configure the RC API server
		opt := rc.Opt // copy global defaults
		opt.Enabled = true
		opt.WebUI = false
		opt.Serve = false

		if command.Flags().Changed("api-addr") {
			opt.HTTP.ListenAddr = apiAddr
		} else {
			port, err := freePort()
			if err != nil {
				return fmt.Errorf("failed to find a free port for RC: %w", err)
			}
			opt.HTTP.ListenAddr = []string{fmt.Sprintf("localhost:%d", port)}
		}

		// CORS: allow the GUI origin to make cross-port API requests.
		opt.HTTP.AllowOrigin = guiOrigin

		// Forward metrics flag to the RC server.
		if command.Flags().Changed("enable-metrics") {
			opt.EnableMetrics = enableMetrics
		}

		// Generate credentials if needed
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
					return fmt.Errorf("failed to make password: %w", err)
				}
				opt.Auth.BasicPass = randomPass
				fs.Infof(nil, "No password specified. Using random password: %s", randomPass)
			}
		}

		// Start the RC server (unchanged rcserver.Start)
		rcServer, err := rcserver.Start(ctx, &opt)
		if err != nil || rcServer == nil {
			return fmt.Errorf("failed to start RC server: %w", err)
		}

		// Build the RC URL from the address we configured (rcserver.Server
		// does not expose URLs, and we know the address we passed in).
		rcURL := "http://" + opt.HTTP.ListenAddr[0] + "/"

		// Mount the GUI handler and start serving
		spaHandler, err := guiHandler(srcFS)
		if err != nil || spaHandler == nil {
			return fmt.Errorf("failed to start GUI handler: %w", err)
		}
		guiServer.Router().Get("/*", spaHandler.ServeHTTP)
		guiServer.Router().Head("/*", spaHandler.ServeHTTP)
		guiServer.Serve()

		guiURL := guiServer.URLs()[0]
		fs.Logf(nil, "Serving GUI on %s", guiURL)

		// Open browser
		loginURL := buildLoginURL(guiURL, rcURL, opt.Auth.BasicUser, opt.Auth.BasicPass, opt.NoAuth)

		fs.Logf(nil, "GUI available at %s", loginURL)
		if flag.Lookup("test.v") == nil && !noOpenBrowser {
			if err := open.Start(loginURL); err != nil {
				fs.Errorf(nil, "Failed to open GUI in browser: %v", err)
			}
		}

		// Wait for either server to exit, then shut both down
		defer systemd.Notify()()
		done := make(chan struct{}, 2)
		go func() { rcServer.Wait(); done <- struct{}{} }()
		go func() { guiServer.Wait(); done <- struct{}{} }()
		<-done
		_ = rcServer.Shutdown()
		_ = guiServer.Shutdown()
		return nil
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

// guiSourceFS opens the GUI bundle at the given path. An empty path
// returns the embedded bundle. The returned cleanup func must be
// called on shutdown (no-op for embedded/DirFS, Close for the zip
// reader).
func guiSourceFS(path string) (iofs.FS, func() error, error) {
	noop := func() error { return nil }
	if path == "" {
		sub, err := iofs.Sub(assets, "dist")
		if err != nil {
			return nil, nil, fmt.Errorf("embedded GUI dir not found: was `make fetch-gui` run before building?: %w", err)
		}
		if _, err := iofs.Stat(sub, "index.html"); err != nil {
			return nil, nil, fmt.Errorf("embedded GUI not found: was `make fetch-gui` run before building?: %w", err)
		}
		return sub, noop, nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to stat GUI source %q: %w", path, err)
	}
	if info.IsDir() {
		return os.DirFS(path), noop, nil
	}
	if info.Mode().IsRegular() && strings.HasSuffix(strings.ToLower(path), ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to open GUI zip %q: %w", path, err)
		}
		return zr, zr.Close, nil
	}
	return nil, nil, fmt.Errorf("GUI source must be a directory or a .zip file: %q", path)
}

// guiHandler returns an http.Handler that serves the GUI bundle from
// srcFS with SPA fallback: paths that don't match a real file return
// index.html.
func guiHandler(srcFS iofs.FS) (http.Handler, error) {
	if _, err := iofs.Stat(srcFS, "index.html"); err != nil {
		return nil, fmt.Errorf("GUI bundle has no index.html: %w", err)
	}
	fileServer := http.FileServer(http.FS(srcFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		if path == "" {
			path = "index.html"
		}
		if _, err := iofs.Stat(srcFS, path); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}
		// SPA fallback: serve index.html for unknown paths so that
		// client-side routing (e.g. /login) works.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	}), nil
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
