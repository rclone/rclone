// Package mcp provides a read-only filesystem interface to a Model
// Context Protocol (MCP) server.
//
// rclone acts as an MCP client. It connects to an MCP server (over stdio,
// streamable HTTP or HTTP+SSE) and presents the server's capabilities as a
// structured, mostly read-only directory tree:
//
//	README.md            server overview, built from the initialize result
//	config.json          connection info and advertised capabilities
//	logs.txt             a snapshot of this session's activity log
//	tools/               one directory per tool
//	  schema.json        every tool and its input schema
//	  <tool>/README.md   documentation, arguments and how to call it
//	  <tool>/schema.json the tool's JSON input schema
//	  <tool>/<tool>      executable wrapper: <tool> --help, <tool> key=value
//	  <tool>/<tool>.sock socket symlink to the backend (with --mcp-sockets)
//	resources/           the server's resources, exposed as readable files
//	  schema.json        every resource (uri, name, mime type, size)
//	  <scheme>/...       resource contents, read via resources/read
//	prompts/             one markdown file per prompt template
//	  schema.json        every prompt and its arguments
//	  <prompt>.md        the prompt's arguments as front matter
//
// Tools and prompts are invoked with the backend command, e.g.
//
//	rclone backend call   remote: <tool> '<json-arguments>'
//	rclone backend prompt remote: <prompt> arg=value
package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
)

// Transport names
const (
	transportAuto  = "auto"
	transportStdio = "stdio"
	transportHTTP  = "http"
	transportSSE   = "sse"
)

var (
	errorReadOnly = errors.New("mcp remotes are read only")
	timeUnset     = time.Unix(0, 0)
)

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "mcp",
		Description: "Model Context Protocol (MCP) server",
		NewFs:       NewFs,
		CommandHelp: commandHelp,
		Options: []fs.Option{{
			Name: "transport",
			Help: `Transport used to talk to the MCP server.

The default "auto" picks "stdio" when a command is configured and
"http" when a url is configured.`,
			Default: transportAuto,
			Examples: []fs.OptionExample{{
				Value: transportAuto,
				Help:  "Auto-detect from the command or url options",
			}, {
				Value: transportStdio,
				Help:  "Run command and talk over its stdin/stdout",
			}, {
				Value: transportHTTP,
				Help:  "Connect to url using the streamable HTTP transport",
			}, {
				Value: transportSSE,
				Help:  "Connect to url using the HTTP+SSE transport (2024-11-05 spec)",
			}},
		}, {
			Name: "command",
			Help: `Command to run the MCP server for the stdio transport.

This is a space separated list of the command and its arguments, e.g.

    npx -y @modelcontextprotocol/server-everything

[CSV encoding](https://godoc.org/encoding/csv) may be used to include
spaces in an argument.`,
		}, {
			Name: "url",
			Help: `URL of the MCP server for the http or sse transport.

E.g. "https://example.com/mcp".`,
		}, {
			Name: "headers",
			Help: `Extra HTTP headers to send for the http and sse transports.

The input format is a comma separated list of key,value pairs.
[CSV encoding](https://godoc.org/encoding/csv) may be used.

For example, to set a custom header use 'X-Example,value'. You can set
multiple headers, e.g. '"X-One","1","X-Two","2"'.`,
			Default:  fs.CommaSepList{},
			Advanced: true,
		}, {
			Name:      "bearer_token",
			Help:      "Bearer token to send in the Authorization header for the http and sse transports.",
			Sensitive: true,
			Advanced:  true,
		}, {
			Name: "sockets",
			Help: `Bind a Unix socket per tool and expose it as a .sock symlink.

For each tool, rclone binds a Unix domain socket in a temporary runtime
directory and exposes it in the tree as "<tool>.sock" (a symlink that
shows up when mounting with --links). Connect to the socket, write the
tool's JSON arguments, then half-close to receive the result, e.g.

    printf '%s' '{"key":"value"}' | nc -U -N tools/<tool>/<tool>.sock

This is intended for use under "rclone mount".`,
			Default:  false,
			Advanced: true,
		}, {
			Name: "socket_dir",
			Help: `Directory to bind the per-tool Unix sockets in.

Implies --mcp-sockets. The sockets are real AF_UNIX socket files (type
"s" in ls -l) that you can connect to directly while rclone is running,
e.g. under a long-lived process such as "rclone mount" or "rclone serve".
If unset, a temporary directory is used and removed on exit.`,
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	Transport   string          `config:"transport"`
	Command     fs.SpaceSepList `config:"command"`
	URL         string          `config:"url"`
	Headers     fs.CommaSepList `config:"headers"`
	BearerToken string          `config:"bearer_token"`
	Sockets     bool            `config:"sockets"`
	SocketDir   string          `config:"socket_dir"`
}

// Fs represents a remote MCP server
type Fs struct {
	name     string
	root     string
	opt      Options
	features *fs.Features
	session  *mcp.ClientSession
	initRes  *mcp.InitializeResult
	tree     *vnode             // synthesized directory tree
	logs     *logBuffer         // activity log behind logs.txt
	bridge   *bridge            // per-tool Unix sockets, nil unless enabled
	ctx      context.Context    // base context for background work
	cancel   context.CancelFunc // cancels ctx on shutdown
}

// vnode is a node in the synthesized directory tree.
//
// A node is either a directory (isDir, children) or a file. A file either
// has static content (data) or is read lazily from an MCP resource (readURI).
type vnode struct {
	name     string
	isDir    bool
	stream   bool // file is the live log stream (logs.txt)
	modTime  time.Time
	mime     string
	size     int64             // file size, -1 if unknown
	data     []byte            // static file content, nil for lazy resources
	readURI  string            // MCP resource URI for lazy reads
	children map[string]*vnode // directory entries
}

func newDir(name string) *vnode {
	return &vnode{name: name, isDir: true, modTime: timeUnset, children: map[string]*vnode{}}
}

func newFile(name string, data []byte, mime string) *vnode {
	return &vnode{name: name, modTime: timeUnset, mime: mime, size: int64(len(data)), data: data}
}

// NewFs creates a new Fs by connecting to the MCP server and building the
// synthesized directory tree from its tools, resources and prompts.
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	if err := configstruct.Set(m, opt); err != nil {
		return nil, err
	}

	f := &Fs{
		name: name,
		root: strings.Trim(root, "/"),
		opt:  *opt,
		logs: newLogBuffer(),
	}
	// Background context for the socket bridge and log streaming, which must
	// outlive any single request.
	f.ctx, f.cancel = context.WithCancel(context.Background())
	f.features = (&fs.Features{}).Fill(ctx, f)

	session, err := f.dial(ctx)
	if err != nil {
		f.cancel()
		return nil, err
	}
	f.session = session
	f.initRes = session.InitializeResult()
	kind, _ := f.transportKind()
	f.logf("connected to %s over %s", f.serverName(), kind)

	// Ask the server to send log notifications if it supports them.
	if caps := f.initRes.Capabilities; caps != nil && caps.Logging != nil {
		if err := session.SetLoggingLevel(ctx, &mcp.SetLoggingLevelParams{Level: "info"}); err != nil {
			fs.Debugf(f, "couldn't set MCP logging level: %v", err)
		}
	}

	f.tree = f.buildTree(ctx)

	// If the root points directly at a file then signal ErrorIsFile.
	if f.root != "" {
		if node := f.resolveAbs(f.root); node != nil && !node.isDir {
			parent, _ := path.Split(f.root)
			f.root = strings.Trim(parent, "/")
			return f, fs.ErrorIsFile
		}
	}

	return f, nil
}

// dial connects to the MCP server and returns an initialized session
func (f *Fs) dial(ctx context.Context) (*mcp.ClientSession, error) {
	transport, err := f.transport(ctx)
	if err != nil {
		return nil, err
	}
	opts := &mcp.ClientOptions{
		LoggingMessageHandler: func(_ context.Context, req *mcp.LoggingMessageRequest) {
			f.logf("[%s] %s", req.Params.Level, formatLogData(req.Params))
		},
	}
	client := mcp.NewClient(&mcp.Implementation{Name: "rclone", Version: fs.Version}, opts)
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("couldn't connect to MCP server: %w", err)
	}
	return session, nil
}

// formatLogData renders an MCP log notification's data as a single line
func formatLogData(p *mcp.LoggingMessageParams) string {
	var msg string
	switch d := p.Data.(type) {
	case string:
		msg = d
	default:
		if b, err := json.Marshal(p.Data); err == nil {
			msg = string(b)
		}
	}
	if p.Logger != "" {
		return p.Logger + ": " + msg
	}
	return msg
}

// transportKind resolves the configured transport, applying auto-detection
func (f *Fs) transportKind() (string, error) {
	switch strings.ToLower(f.opt.Transport) {
	case transportStdio, transportHTTP, transportSSE:
		return strings.ToLower(f.opt.Transport), nil
	case "", transportAuto:
		switch {
		case len(f.opt.Command) > 0:
			return transportStdio, nil
		case f.opt.URL != "":
			return transportHTTP, nil
		default:
			return "", errors.New("mcp: set \"command\" for the stdio transport or \"url\" for the http/sse transport")
		}
	default:
		return "", fmt.Errorf("mcp: unknown transport %q", f.opt.Transport)
	}
}

// transport builds the MCP transport from the configured options
func (f *Fs) transport(ctx context.Context) (mcp.Transport, error) {
	kind, err := f.transportKind()
	if err != nil {
		return nil, err
	}
	switch kind {
	case transportStdio:
		if len(f.opt.Command) == 0 {
			return nil, errors.New("mcp: \"command\" is required for the stdio transport")
		}
		args := []string(f.opt.Command)
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Stderr = stderrLogger{f: f}
		return &mcp.CommandTransport{Command: cmd}, nil
	default: // http or sse
		if f.opt.URL == "" {
			return nil, errors.New("mcp: \"url\" is required for the http and sse transports")
		}
		client, err := f.httpClient(ctx)
		if err != nil {
			return nil, err
		}
		if kind == transportSSE {
			return &mcp.SSEClientTransport{Endpoint: f.opt.URL, HTTPClient: client}, nil
		}
		// We only issue request/response calls, so there's no need for the
		// persistent server->client SSE stream.
		return &mcp.StreamableClientTransport{
			Endpoint:             f.opt.URL,
			HTTPClient:           client,
			DisableStandaloneSSE: true,
		}, nil
	}
}

// httpClient returns an HTTP client that injects the configured headers
func (f *Fs) httpClient(ctx context.Context) (*http.Client, error) {
	headers, err := headerPairs(f.opt.Headers)
	if err != nil {
		return nil, err
	}
	if f.opt.BearerToken != "" {
		headers = append(headers, [2]string{"Authorization", "Bearer " + f.opt.BearerToken})
	}
	client := fshttp.NewClient(ctx)
	if len(headers) > 0 {
		base := client.Transport
		if base == nil {
			base = http.DefaultTransport
		}
		client.Transport = &headerRoundTripper{base: base, headers: headers}
	}
	return client, nil
}

// headerPairs converts a flat key,value list into pairs
func headerPairs(list fs.CommaSepList) ([][2]string, error) {
	if len(list)%2 != 0 {
		return nil, errors.New("mcp: headers should be a list of key,value pairs")
	}
	pairs := make([][2]string, 0, len(list)/2)
	for i := 0; i < len(list); i += 2 {
		pairs = append(pairs, [2]string{list[i], list[i+1]})
	}
	return pairs, nil
}

// headerRoundTripper adds fixed headers to every request
type headerRoundTripper struct {
	base    http.RoundTripper
	headers [][2]string
}

// RoundTrip implements http.RoundTripper
func (h *headerRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for _, kv := range h.headers {
		req.Header.Set(kv[0], kv[1])
	}
	return h.base.RoundTrip(req)
}

// stderrLogger forwards an MCP subprocess's stderr to the debug log
type stderrLogger struct{ f *Fs }

// Write implements io.Writer
func (w stderrLogger) Write(p []byte) (int, error) {
	for _, line := range strings.Split(strings.TrimRight(string(p), "\n"), "\n") {
		if line != "" {
			fs.Debugf(w.f, "server: %s", line)
			w.f.logf("server: %s", line)
		}
	}
	return len(p), nil
}

// buildTree queries the server and builds the synthesized directory tree.
//
// Enumeration errors for a section are logged and that section is left
// empty rather than failing the whole remote.
func (f *Fs) buildTree(ctx context.Context) *vnode {
	root := newDir("")
	root.children["README.md"] = newFile("README.md", f.renderReadme(), "text/markdown")
	root.children["config.json"] = newFile("config.json", f.renderConfig(), "application/json")
	root.children["logs.txt"] = &vnode{name: "logs.txt", stream: true, modTime: timeUnset, mime: "text/plain", size: -1}

	caps := f.initRes.Capabilities
	if caps != nil && caps.Tools != nil {
		tools := f.collectTools(ctx)
		// Optionally bind a Unix socket per tool.
		if (f.opt.Sockets || f.opt.SocketDir != "") && len(tools) > 0 {
			if err := f.startBridge(); err != nil {
				fs.Logf(f, "couldn't start MCP socket bridge: %v", err)
			}
		}
		root.children["tools"] = f.buildToolsDir(tools)
	}
	if caps != nil && caps.Resources != nil {
		root.children["resources"] = f.buildResources(ctx)
	}
	if caps != nil && caps.Prompts != nil {
		root.children["prompts"] = f.buildPrompts(ctx)
	}
	return root
}

// collectTools lists the server's tools, sorted by name
func (f *Fs) collectTools(ctx context.Context) []*mcp.Tool {
	var tools []*mcp.Tool
	for t, err := range f.session.Tools(ctx, nil) {
		if err != nil {
			fs.Logf(f, "couldn't list MCP tools: %v", err)
			break
		}
		tools = append(tools, t)
	}
	sort.Slice(tools, func(i, j int) bool { return tools[i].Name < tools[j].Name })
	return tools
}

// buildToolsDir builds the tools/ directory. Each tool gets its own
// directory containing its documentation, an executable wrapper named after
// the tool, and (with --mcp-sockets) a "<tool>.sock" symlink to its socket.
func (f *Fs) buildToolsDir(tools []*mcp.Tool) *vnode {
	dir := newDir("tools")
	dir.children["schema.json"] = newFile("schema.json", marshalJSON(tools), "application/json")
	for _, t := range tools {
		name := uniqueName(dir.children, sanitizeSegment(t.Name))
		td := newDir(name)
		td.children["README.md"] = newFile("README.md", f.renderToolReadme(t), "text/markdown")
		td.children["schema.json"] = newFile("schema.json", marshalJSON(t.InputSchema), "application/json")
		// Executable wrapper named after the tool. It is a regular file;
		// mount with --file-perms 0755 (or run it with bash) to execute it.
		wname := uniqueName(td.children, name)
		td.children[wname] = newFile(wname, f.renderToolWrapper(t), "text/x-shellscript")
		// A socket exposed as a symlink (shows up with --links on mount).
		if f.bridge != nil {
			if sockPath := f.bridge.bind(f, t.Name); sockPath != "" {
				linkName := uniqueName(td.children, name+".sock"+fs.LinkSuffix)
				td.children[linkName] = newFile(linkName, []byte(sockPath), "inode/symlink")
			}
		}
		dir.children[name] = td
	}
	return dir
}

// buildResources builds the resources/ directory
func (f *Fs) buildResources(ctx context.Context) *vnode {
	dir := newDir("resources")
	var resources []*mcp.Resource
	for r, err := range f.session.Resources(ctx, nil) {
		if err != nil {
			fs.Logf(f, "couldn't list MCP resources: %v", err)
			break
		}
		resources = append(resources, r)
	}
	sort.Slice(resources, func(i, j int) bool { return resources[i].URI < resources[j].URI })
	dir.children["schema.json"] = newFile("schema.json", marshalJSON(resources), "application/json")
	// If every resource shares one URI scheme, drop the scheme from the path
	// so resources sit directly under resources/. With mixed schemes we keep
	// it as a top level directory to avoid collisions.
	dropScheme := singleScheme(resources)
	for _, r := range resources {
		p := resourcePath(r.URI, dropScheme)
		if p == "" {
			fs.Debugf(f, "ignoring resource with unmappable URI %q", r.URI)
			continue
		}
		size := int64(-1)
		if r.Size > 0 {
			size = r.Size
		}
		leaf := &vnode{modTime: timeUnset, mime: r.MIMEType, size: size, readURI: r.URI}
		addPath(dir, strings.Split(p, "/"), leaf)
	}
	return dir
}

// singleScheme reports whether all resources share a single non-empty URI
// scheme
func singleScheme(resources []*mcp.Resource) bool {
	scheme := ""
	for _, r := range resources {
		u, err := url.Parse(r.URI)
		if err != nil || u.Scheme == "" {
			return false
		}
		if scheme == "" {
			scheme = u.Scheme
		} else if scheme != u.Scheme {
			return false
		}
	}
	return scheme != ""
}

// buildPrompts builds the prompts/ directory
func (f *Fs) buildPrompts(ctx context.Context) *vnode {
	dir := newDir("prompts")
	var prompts []*mcp.Prompt
	for p, err := range f.session.Prompts(ctx, nil) {
		if err != nil {
			fs.Logf(f, "couldn't list MCP prompts: %v", err)
			break
		}
		prompts = append(prompts, p)
	}
	sort.Slice(prompts, func(i, j int) bool { return prompts[i].Name < prompts[j].Name })
	dir.children["schema.json"] = newFile("schema.json", marshalJSON(prompts), "application/json")
	for _, p := range prompts {
		name := uniqueName(dir.children, sanitizeSegment(p.Name)+".md")
		dir.children[name] = newFile(name, f.renderPrompt(p), "text/markdown")
	}
	return dir
}

// addPath inserts leaf into the tree rooted at dir, creating intermediate
// directories. File/dir and name collisions are resolved by suffixing.
func addPath(dir *vnode, segments []string, leaf *vnode) {
	cur := dir
	for _, seg := range segments[:len(segments)-1] {
		child := cur.children[seg]
		if child == nil {
			child = newDir(seg)
			cur.children[seg] = child
		} else if !child.isDir {
			// A file is where we need a directory: move the file aside.
			alt := uniqueName(cur.children, child.name+"~file")
			child.name = alt
			cur.children[alt] = child
			child = newDir(seg)
			cur.children[seg] = child
		}
		cur = child
	}
	name := uniqueName(cur.children, segments[len(segments)-1])
	leaf.name = name
	cur.children[name] = leaf
}

// uniqueName returns base, or base with a numeric suffix, not already used
func uniqueName(children map[string]*vnode, base string) string {
	if _, ok := children[base]; !ok {
		return base
	}
	for n := 2; ; n++ {
		cand := fmt.Sprintf("%s-%d", base, n)
		if _, ok := children[cand]; !ok {
			return cand
		}
	}
}

// resourcePath maps an MCP resource URI to a slash separated path. Unless
// dropScheme is set, the URI scheme becomes the top level directory so
// resources from different schemes don't collide. Returns "" if the URI
// can't be mapped.
func resourcePath(uri string, dropScheme bool) string {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme == "" {
		return sanitizeSegment(uri)
	}
	var segs []string
	if !dropScheme {
		segs = append(segs, u.Scheme)
	}
	if u.Host != "" {
		segs = append(segs, sanitizeSegment(u.Host))
	}
	p := u.Path
	if p == "" {
		p = u.Opaque
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" {
			continue
		}
		if unescaped, err := url.PathUnescape(seg); err == nil {
			seg = unescaped
		}
		if s := sanitizeSegment(seg); s != "" {
			segs = append(segs, s)
		}
	}
	return strings.Join(segs, "/")
}

// sanitizeSegment makes a single path segment safe to use as a file name
func sanitizeSegment(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\x00", "")
	switch s {
	case "", ".", "..":
		return "_" + s
	}
	return s
}

// marshalJSON pretty-prints v as JSON with a trailing newline
func marshalJSON(v any) []byte {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return []byte(fmt.Sprintf("%v\n", err))
	}
	return append(data, '\n')
}

// serverName returns a display name for the server
func (f *Fs) serverName() string {
	if f.initRes != nil && f.initRes.ServerInfo != nil && f.initRes.ServerInfo.Name != "" {
		return f.initRes.ServerInfo.Name
	}
	return f.name
}

// resolveAbs walks the tree to the node at the slash separated absolute path
func (f *Fs) resolveAbs(abs string) *vnode {
	abs = strings.Trim(abs, "/")
	cur := f.tree
	if abs == "" {
		return cur
	}
	for _, seg := range strings.Split(abs, "/") {
		if cur == nil || !cur.isDir {
			return nil
		}
		cur = cur.children[seg]
	}
	return cur
}

// node walks the tree to the node at a remote-relative path
func (f *Fs) node(remote string) *vnode {
	return f.resolveAbs(path.Join(f.root, remote))
}

// Name returns the configured name of the file system
func (f *Fs) Name() string { return f.name }

// Root returns the root for the filesystem
func (f *Fs) Root() string { return f.root }

// String returns a description of the FS
func (f *Fs) String() string {
	if len(f.opt.Command) > 0 {
		return fmt.Sprintf("MCP server %q", strings.Join([]string(f.opt.Command), " "))
	}
	return fmt.Sprintf("MCP server %s", f.opt.URL)
}

// Precision is not supported as MCP resources have no modification time
func (f *Fs) Precision() time.Duration { return fs.ModTimeNotSupported }

// Hashes returns the supported hash types of the filesystem
func (f *Fs) Hashes() hash.Set { return hash.Set(hash.None) }

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features { return f.features }

// List the objects and directories in dir into entries.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	node := f.node(dir)
	if node == nil || !node.isDir {
		return nil, fs.ErrorDirNotFound
	}
	dir = strings.Trim(dir, "/")
	for name, child := range node.children {
		remote := name
		if dir != "" {
			remote = dir + "/" + name
		}
		if child.isDir {
			entries = append(entries, fs.NewDir(remote, child.modTime))
		} else {
			entries = append(entries, f.newObject(remote, child))
		}
	}
	return entries, nil
}

// NewObject finds the Object at remote.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	node := f.node(remote)
	if node == nil {
		return nil, fs.ErrorObjectNotFound
	}
	if node.isDir {
		return nil, fs.ErrorIsDir
	}
	return f.newObject(strings.Trim(remote, "/"), node), nil
}

// newObject builds an Object from a tree node
func (f *Fs) newObject(remote string, node *vnode) *Object {
	return &Object{fs: f, remote: remote, node: node}
}

// Put is not supported as the MCP filesystem is read only.
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return nil, errorReadOnly
}

// Mkdir is not supported as the MCP filesystem is read only.
func (f *Fs) Mkdir(ctx context.Context, dir string) error { return errorReadOnly }

// Rmdir is not supported as the MCP filesystem is read only.
func (f *Fs) Rmdir(ctx context.Context, dir string) error { return errorReadOnly }

// Shutdown closes the MCP session, the socket bridge and the log stream,
// terminating any subprocess.
func (f *Fs) Shutdown(ctx context.Context) error {
	if f.cancel != nil {
		f.cancel()
	}
	if f.bridge != nil {
		f.bridge.close()
	}
	if f.session == nil {
		return nil
	}
	return f.session.Close()
}

// Object describes a node in the MCP filesystem
type Object struct {
	fs     *Fs
	remote string
	node   *vnode
}

// Fs returns read only access to the Fs that this object is part of
func (o *Object) Fs() fs.Info { return o.fs }

// String returns a description of the Object
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string { return o.remote }

// ModTime returns the modification date of the file
func (o *Object) ModTime(ctx context.Context) time.Time { return o.node.modTime }

// Size returns the size of the file, or -1 if it isn't known
func (o *Object) Size() int64 { return o.node.size }

// Hash is not supported so it always returns ""
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) { return "", nil }

// Storable says whether this object can be stored
func (o *Object) Storable() bool { return true }

// MimeType returns the content type of the Object if known
func (o *Object) MimeType(ctx context.Context) string { return o.node.mime }

// SetModTime is not supported as the MCP filesystem is read only.
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorCantSetModTime
}

// Open opens the file for read.
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	// logs.txt serves a snapshot of the activity log.
	if o.node.stream {
		return readerForRange(o.fs.logs.snapshot(), options), nil
	}
	data := o.node.data
	if data == nil && o.node.readURI != "" {
		res, err := o.fs.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: o.node.readURI})
		if err != nil {
			return nil, fmt.Errorf("couldn't read MCP resource %q: %w", o.node.readURI, err)
		}
		for _, c := range res.Contents {
			if len(c.Blob) > 0 {
				data = append(data, c.Blob...)
			} else {
				data = append(data, c.Text...)
			}
		}
	}
	return readerForRange(data, options), nil
}

// readerForRange returns a reader over data honouring any range/seek options
func readerForRange(data []byte, options []fs.OpenOption) io.ReadCloser {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.RangeOption:
			offset, limit = x.Decode(int64(len(data)))
		case *fs.SeekOption:
			offset = x.Offset
		default:
			if option.Mandatory() {
				fs.Logf(nil, "Unsupported mandatory option: %v", option)
			}
		}
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	data = data[offset:]
	if limit >= 0 && limit < int64(len(data)) {
		data = data[:limit]
	}
	return io.NopCloser(bytes.NewReader(data))
}

// Update is not supported as the MCP filesystem is read only.
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	return errorReadOnly
}

// Remove is not supported as the MCP filesystem is read only.
func (o *Object) Remove(ctx context.Context) error { return errorReadOnly }

// ------------------------------------------------------------
// Content rendering
// ------------------------------------------------------------

// renderReadme builds the top level README.md from the initialize result
func (f *Fs) renderReadme() []byte {
	var b strings.Builder
	info := f.initRes
	fmt.Fprintf(&b, "# %s\n\n", f.serverName())
	if info != nil && info.ServerInfo != nil {
		if info.ServerInfo.Title != "" {
			fmt.Fprintf(&b, "%s\n\n", info.ServerInfo.Title)
		}
		fmt.Fprintf(&b, "- Server: %s %s\n", info.ServerInfo.Name, info.ServerInfo.Version)
	}
	if info != nil && info.ProtocolVersion != "" {
		fmt.Fprintf(&b, "- Protocol version: %s\n", info.ProtocolVersion)
	}
	if caps := capabilityNames(info); len(caps) > 0 {
		fmt.Fprintf(&b, "- Capabilities: %s\n", strings.Join(caps, ", "))
	}
	b.WriteString("\n")
	if info != nil && info.Instructions != "" {
		b.WriteString("## Instructions\n\n")
		b.WriteString(info.Instructions)
		b.WriteString("\n\n")
	}
	b.WriteString("## Layout\n\n")
	b.WriteString("- `tools/` - one directory per tool, each with a README, JSON\n")
	b.WriteString("  schema, an executable wrapper and (with --mcp-sockets) a socket\n")
	b.WriteString("- `resources/` - the server's resources, exposed as readable files\n")
	b.WriteString("- `prompts/` - the server's prompt templates\n\n")
	b.WriteString("Run a tool directly (mount with --file-perms 0755 to execute it):\n\n")
	b.WriteString("    tools/<tool>/<tool> --help\n")
	b.WriteString("    tools/<tool>/<tool> key=value\n\n")
	b.WriteString("Or invoke it with the backend command:\n\n")
	fmt.Fprintf(&b, "    rclone backend call %s: <tool> key=value\n", f.name)
	fmt.Fprintf(&b, "    rclone backend prompt %s: <prompt> arg=value\n", f.name)
	return []byte(b.String())
}

// capabilityNames returns the names of the capabilities the server advertises
func capabilityNames(info *mcp.InitializeResult) (names []string) {
	if info == nil || info.Capabilities == nil {
		return nil
	}
	c := info.Capabilities
	if c.Tools != nil {
		names = append(names, "tools")
	}
	if c.Resources != nil {
		names = append(names, "resources")
	}
	if c.Prompts != nil {
		names = append(names, "prompts")
	}
	if c.Logging != nil {
		names = append(names, "logging")
	}
	return names
}

// renderConfig builds the top level config.json
func (f *Fs) renderConfig() []byte {
	kind, _ := f.transportKind()
	type capsJSON struct {
		Tools     bool `json:"tools"`
		Resources bool `json:"resources"`
		Prompts   bool `json:"prompts"`
		Logging   bool `json:"logging"`
	}
	var cj capsJSON
	if c := f.initRes.Capabilities; c != nil {
		cj.Tools = c.Tools != nil
		cj.Resources = c.Resources != nil
		cj.Prompts = c.Prompts != nil
		cj.Logging = c.Logging != nil
	}
	cfg := struct {
		Name            string              `json:"name"`
		Transport       string              `json:"transport"`
		ProtocolVersion string              `json:"protocol_version,omitempty"`
		Server          *mcp.Implementation `json:"server,omitempty"`
		Capabilities    capsJSON            `json:"capabilities"`
	}{
		Name:         f.name,
		Transport:    kind,
		Capabilities: cj,
	}
	if f.initRes != nil {
		cfg.ProtocolVersion = f.initRes.ProtocolVersion
		cfg.Server = f.initRes.ServerInfo
	}
	return marshalJSON(cfg)
}

// renderToolReadme builds the README.md for a single tool
func (f *Fs) renderToolReadme(t *mcp.Tool) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", t.Name)
	if t.Title != "" {
		fmt.Fprintf(&b, "%s\n\n", t.Title)
	}
	if t.Description != "" {
		b.WriteString(t.Description)
		b.WriteString("\n\n")
	}
	b.WriteString("## Input schema\n\n```json\n")
	b.Write(bytes.TrimRight(marshalJSON(t.InputSchema), "\n"))
	b.WriteString("\n```\n\n")
	b.WriteString("## Call\n\n")
	fmt.Fprintf(&b, "    rclone backend call %s: %s '<json-arguments>'\n\n", f.name, t.Name)
	b.WriteString("or run the wrapper script in this directory:\n\n")
	fmt.Fprintf(&b, "    bash %s '<json-arguments>'\n", t.Name)
	return []byte(b.String())
}

// renderToolWrapper builds the executable bash wrapper for a tool. The
// wrapper is self-contained: "<tool> --help" prints the schema and "<tool>
// key=value" or "<tool> '{json}'" calls the tool, so a user (or a small LLM
// driving bash) needs no knowledge of MCP or rclone internals.
func (f *Fs) renderToolWrapper(t *mcp.Tool) []byte {
	desc := t.Description
	if desc == "" {
		desc = t.Name
	}
	schema := strings.TrimRight(string(marshalJSON(t.InputSchema)), "\n")
	var b strings.Builder
	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n")
	b.WriteString("if [ \"${1:-}\" = \"-h\" ] || [ \"${1:-}\" = \"--help\" ]; then\n")
	b.WriteString("cat <<'MCPHELP'\n")
	fmt.Fprintf(&b, "%s - %s\n\n", t.Name, desc)
	b.WriteString("Usage:\n")
	fmt.Fprintf(&b, "  %s key=value [key=value ...]\n", t.Name)
	fmt.Fprintf(&b, "  %s '{\"key\": \"value\"}'\n\n", t.Name)
	b.WriteString("Input schema:\n")
	b.WriteString(schema)
	b.WriteString("\nMCPHELP\n")
	b.WriteString("exit 0\nfi\n")
	fmt.Fprintf(&b, "exec rclone backend call %s: %s \"$@\"\n", f.name, t.Name)
	return []byte(b.String())
}

// renderPrompt builds the markdown file for a single prompt template
func (f *Fs) renderPrompt(p *mcp.Prompt) []byte {
	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "name: %s\n", p.Name)
	if p.Title != "" {
		fmt.Fprintf(&b, "title: %s\n", p.Title)
	}
	if len(p.Arguments) > 0 {
		b.WriteString("arguments:\n")
		for _, a := range p.Arguments {
			fmt.Fprintf(&b, "  - name: %s\n", a.Name)
			fmt.Fprintf(&b, "    required: %t\n", a.Required)
			if a.Description != "" {
				fmt.Fprintf(&b, "    description: %s\n", a.Description)
			}
		}
	}
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "# %s\n\n", p.Name)
	if p.Description != "" {
		b.WriteString(p.Description)
		b.WriteString("\n\n")
	}
	b.WriteString("Render this prompt with:\n\n")
	fmt.Fprintf(&b, "    rclone backend prompt %s: %s", f.name, p.Name)
	for _, a := range p.Arguments {
		fmt.Fprintf(&b, " %s=value", a.Name)
	}
	b.WriteString("\n")
	return []byte(b.String())
}

// renderContent renders a list of MCP content blocks to text
func renderContent(contents []mcp.Content) string {
	parts := make([]string, 0, len(contents))
	for _, c := range contents {
		switch tc := c.(type) {
		case *mcp.TextContent:
			parts = append(parts, tc.Text)
		default:
			if data, err := c.MarshalJSON(); err == nil {
				parts = append(parts, string(data))
			}
		}
	}
	return strings.Join(parts, "\n")
}

// renderPromptMessages renders the result of a prompts/get call
func renderPromptMessages(res *mcp.GetPromptResult) string {
	var b strings.Builder
	if res.Description != "" {
		b.WriteString(res.Description)
		b.WriteString("\n\n")
	}
	for _, m := range res.Messages {
		fmt.Fprintf(&b, "[%s]\n", m.Role)
		b.WriteString(renderContent([]mcp.Content{m.Content}))
		b.WriteString("\n")
	}
	return b.String()
}

// ------------------------------------------------------------
// Backend command
// ------------------------------------------------------------

var commandHelp = []fs.CommandHelp{{
	Name:  "call",
	Short: "Call an MCP tool and print its result.",
	Long: `Call a tool on the MCP server and print its result.

The first argument is the tool name. The arguments may be given either as a
single JSON object, or as key=value pairs (each value is parsed as JSON if
possible, so numbers and booleans work, otherwise it is a string):

    rclone backend call remote: search_repo '{"query":"rclone","limit":5}'
    rclone backend call remote: search_repo query=rclone limit=5
`,
}, {
	Name:  "install",
	Short: "Write executable wrappers for every tool into a directory.",
	Long: `Write an executable wrapper script for each tool into a directory,
turning the MCP server's tools into ordinary command line programs:

    rclone backend install remote: ~/bin/mcp-tools

Then run a tool directly, with no knowledge of MCP required:

    ~/bin/mcp-tools/search_repo --help
    ~/bin/mcp-tools/search_repo query=rclone limit=5

The wrappers shell out to "rclone backend call", so rclone must be on the
PATH and the remote must stay configured.
`,
}, {
	Name:  "prompt",
	Short: "Render an MCP prompt template.",
	Long: `Get a prompt from the MCP server and print its messages.

The first argument is the prompt name. Any further arguments are key=value
pairs (you can also use -o key=value), e.g.

    rclone backend prompt remote: code_review file=main.go
`,
}, {
	Name:  "read",
	Short: "Read an MCP resource and print its contents.",
	Long: `Read a resource and print its contents.

The argument is either a resource URI or a path under resources/, e.g.

    rclone backend read remote: file:///etc/hosts
    rclone backend read remote: resources/file/etc/hosts
`,
}, {
	Name:  "tools",
	Short: "List the names of the tools the server exposes.",
}, {
	Name:  "prompts",
	Short: "List the names of the prompts the server exposes.",
}, {
	Name:  "resources",
	Short: "List the URIs of the resources the server exposes.",
}}

// Command implements the backend command interface for invoking the server
func (f *Fs) Command(ctx context.Context, name string, arg []string, opt map[string]string) (any, error) {
	switch name {
	case "call":
		if len(arg) < 1 {
			return nil, errors.New("need a tool name")
		}
		args, err := buildToolArgs(arg[1:], opt)
		if err != nil {
			return nil, err
		}
		f.logf("call tool %s", arg[0])
		res, err := f.session.CallTool(ctx, &mcp.CallToolParams{Name: arg[0], Arguments: args})
		if err != nil {
			return nil, err
		}
		out := renderContent(res.Content)
		if out == "" && res.StructuredContent != nil {
			out = string(bytes.TrimRight(marshalJSON(res.StructuredContent), "\n"))
		}
		if res.IsError {
			return out, errors.New("tool reported an error")
		}
		return out, nil
	case "prompt":
		if len(arg) < 1 {
			return nil, errors.New("need a prompt name")
		}
		args := map[string]string{}
		for k, v := range opt {
			args[k] = v
		}
		for _, a := range arg[1:] {
			if k, v, ok := strings.Cut(a, "="); ok {
				args[k] = v
			}
		}
		res, err := f.session.GetPrompt(ctx, &mcp.GetPromptParams{Name: arg[0], Arguments: args})
		if err != nil {
			return nil, err
		}
		return renderPromptMessages(res), nil
	case "read":
		if len(arg) < 1 {
			return nil, errors.New("need a resource uri or path")
		}
		uri := arg[0]
		if node := f.node(arg[0]); node != nil && node.readURI != "" {
			uri = node.readURI
		}
		res, err := f.session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		for _, c := range res.Contents {
			if len(c.Blob) > 0 {
				b.Write(c.Blob)
			} else {
				b.WriteString(c.Text)
			}
		}
		return b.String(), nil
	case "tools", "prompts":
		return f.topNames(name), nil
	case "resources":
		node := f.resolveAbs("resources")
		var uris []string
		if node != nil {
			collectURIs(node, &uris)
		}
		sort.Strings(uris)
		return uris, nil
	case "install":
		if len(arg) < 1 {
			return nil, errors.New("need a target directory")
		}
		return f.installTools(arg[0])
	default:
		return nil, fs.ErrorCommandNotFound
	}
}

// buildToolArgs turns command line arguments into MCP tool arguments. A
// single "{...}" or "[...]" argument is parsed as JSON; otherwise the
// arguments (and any -o options) are treated as key=value pairs, with each
// value parsed as JSON if it can be (so n=5 is a number, ok=true a bool)
// and used as a plain string otherwise.
func buildToolArgs(rest []string, opt map[string]string) (any, error) {
	if len(rest) == 1 && len(opt) == 0 {
		s := strings.TrimSpace(rest[0])
		if strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[") {
			var v any
			if err := json.Unmarshal([]byte(s), &v); err != nil {
				return nil, fmt.Errorf("invalid JSON arguments: %w", err)
			}
			return v, nil
		}
	}
	m := map[string]any{}
	for _, kv := range rest {
		k, v, ok := strings.Cut(kv, "=")
		if !ok {
			return nil, fmt.Errorf("argument %q is not key=value or a JSON object", kv)
		}
		m[k] = inferValue(v)
	}
	for k, v := range opt {
		m[k] = inferValue(v)
	}
	if len(m) == 0 {
		return nil, nil
	}
	return m, nil
}

// inferValue parses s as a JSON scalar/array/object if possible, otherwise
// returns it as a plain string
func inferValue(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

// installTools writes the executable tool wrappers into dir so the tools
// can be used as ordinary command line programs
func (f *Fs) installTools(dir string) (any, error) {
	toolsNode := f.resolveAbs("tools")
	if toolsNode == nil {
		return nil, errors.New("this MCP server exposes no tools")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	var installed []string
	for name, td := range toolsNode.children {
		if !td.isDir {
			continue
		}
		script := td.children[name] // the wrapper is named after the tool
		if script == nil || script.data == nil {
			continue
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, script.data, 0o755); err != nil {
			return nil, err
		}
		if err := os.Chmod(path, 0o755); err != nil {
			return nil, err
		}
		installed = append(installed, name)
	}
	sort.Strings(installed)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), f.renderToolboxReadme(installed), 0o644); err != nil {
		return nil, err
	}
	return map[string]any{"directory": dir, "tools": installed}, nil
}

// renderToolboxReadme documents an installed toolbox directory
func (f *Fs) renderToolboxReadme(tools []string) []byte {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s tools\n\n", f.serverName())
	b.WriteString("Each file here is an executable wrapper for one MCP tool.\n\n")
	b.WriteString("    ./<tool> --help            # show the tool's input schema\n")
	b.WriteString("    ./<tool> key=value ...     # call it with arguments\n")
	b.WriteString("    ./<tool> '{\"key\":\"value\"}' # or with a JSON object\n\n")
	b.WriteString("Tools:\n\n")
	for _, t := range tools {
		fmt.Fprintf(&b, "- %s\n", t)
	}
	return []byte(b.String())
}

// topNames returns the names of a directory's children, dropping schema.json
// and trimming any ".md" suffix
func (f *Fs) topNames(dir string) []string {
	node := f.resolveAbs(dir)
	if node == nil {
		return []string{}
	}
	names := make([]string, 0, len(node.children))
	for n := range node.children {
		if n == "schema.json" {
			continue
		}
		names = append(names, strings.TrimSuffix(n, ".md"))
	}
	sort.Strings(names)
	return names
}

// collectURIs walks the tree appending the URIs of any resource files
func collectURIs(node *vnode, out *[]string) {
	for _, c := range node.children {
		if c.isDir {
			collectURIs(c, out)
		} else if c.readURI != "" {
			*out = append(*out, c.readURI)
		}
	}
}

// ------------------------------------------------------------
// Activity log (logs.txt)
// ------------------------------------------------------------

// logHistory is the maximum number of log lines retained
const logHistory = 256

// logBuffer accumulates a bounded history of activity lines.
//
// logs.txt serves a snapshot of this buffer rather than a never-ending
// stream: rclone's transfer layer reads ahead with a fixed-size buffer and
// expects finite objects, so an infinite stream would block until the buffer
// filled. A snapshot reflecting everything up to read time composes cleanly
// with cat, copy and mount, and reading again shows any newer entries.
type logBuffer struct {
	mu      sync.Mutex
	history [][]byte
}

func newLogBuffer() *logBuffer { return &logBuffer{} }

// write records a line, dropping the oldest once the history is full
func (b *logBuffer) write(line []byte) {
	b.mu.Lock()
	b.history = append(b.history, line)
	if len(b.history) > logHistory {
		b.history = b.history[len(b.history)-logHistory:]
	}
	b.mu.Unlock()
}

// snapshot returns the current history as a single byte slice
func (b *logBuffer) snapshot() []byte {
	b.mu.Lock()
	defer b.mu.Unlock()
	var out []byte
	for _, line := range b.history {
		out = append(out, line...)
	}
	return out
}

// logf records a formatted, timestamped line on the activity log
func (f *Fs) logf(format string, args ...any) {
	if f.logs == nil {
		return
	}
	line := time.Now().UTC().Format("2006-01-02T15:04:05Z") + " " + fmt.Sprintf(format, args...) + "\n"
	f.logs.write([]byte(line))
}

// ------------------------------------------------------------
// Per-tool Unix socket bridge
// ------------------------------------------------------------

// maxSocketRequest caps how many bytes of arguments a socket client may send
const maxSocketRequest = 1 << 20

// bridge binds a Unix socket per tool in a runtime directory and proxies
// connections to tools/call.
type bridge struct {
	dir       string
	ownDir    bool // true if we created dir and should remove it
	mu        sync.Mutex
	listeners []net.Listener
}

// startBridge prepares the runtime directory for the socket bridge, using
// the configured socket_dir if set, otherwise a temporary directory.
func (f *Fs) startBridge() error {
	if dir := f.opt.SocketDir; dir != "" {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return err
		}
		f.bridge = &bridge{dir: dir, ownDir: false}
		return nil
	}
	dir, err := os.MkdirTemp("", "rclone-mcp-")
	if err != nil {
		return err
	}
	f.bridge = &bridge{dir: dir, ownDir: true}
	return nil
}

// bind binds a socket for tool and starts serving it, returning its path
func (b *bridge) bind(f *Fs, tool string) string {
	sockPath := filepath.Join(b.dir, sanitizeSegment(tool)+".sock")
	l, err := net.Listen("unix", sockPath)
	if err != nil {
		fs.Logf(f, "couldn't bind socket for tool %q: %v", tool, err)
		return ""
	}
	b.mu.Lock()
	b.listeners = append(b.listeners, l)
	b.mu.Unlock()
	go f.serveSocket(l, tool)
	return sockPath
}

// close stops all listeners (which unlinks the socket files) and removes
// the runtime directory if we created it.
func (b *bridge) close() {
	b.mu.Lock()
	for _, l := range b.listeners {
		_ = l.Close()
	}
	b.listeners = nil
	b.mu.Unlock()
	if b.ownDir {
		_ = os.RemoveAll(b.dir)
	}
}

// serveSocket accepts connections for a tool's socket until it is closed
func (f *Fs) serveSocket(l net.Listener, tool string) {
	for {
		conn, err := l.Accept()
		if err != nil {
			return // listener closed
		}
		go f.handleSocketConn(conn, tool)
	}
}

// handleSocketConn reads JSON arguments, calls the tool and writes the result
func (f *Fs) handleSocketConn(conn net.Conn, tool string) {
	defer func() { _ = conn.Close() }()
	reqData, err := io.ReadAll(io.LimitReader(conn, maxSocketRequest))
	if err != nil {
		fmt.Fprintf(conn, "error: %v\n", err)
		return
	}
	var args any
	if len(bytes.TrimSpace(reqData)) > 0 {
		if err := json.Unmarshal(reqData, &args); err != nil {
			fmt.Fprintf(conn, "error: invalid JSON arguments: %v\n", err)
			return
		}
	}
	f.logf("call tool %s (socket)", tool)
	res, err := f.session.CallTool(f.ctx, &mcp.CallToolParams{Name: tool, Arguments: args})
	if err != nil {
		fmt.Fprintf(conn, "error: %v\n", err)
		return
	}
	_, _ = io.WriteString(conn, renderContent(res.Content))
}

// Check the interfaces are satisfied
var (
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
	_ fs.Commander  = (*Fs)(nil)
	_ fs.Object     = (*Object)(nil)
	_ fs.MimeTyper  = (*Object)(nil)
)
