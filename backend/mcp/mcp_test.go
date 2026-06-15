package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testServerEnv = "RCLONE_MCP_TEST_SERVER"

// testRes is a resource exposed by the test MCP server
type testRes struct {
	text string
	size int64 // 0 means don't advertise a size
}

var testResources = map[string]testRes{
	"file:///a.txt":         {text: "alpha", size: 5},
	"file:///dir/b.txt":     {text: "bravo"},
	"file:///dir/sub/c.txt": {text: "charlie"},
	"config://settings":     {text: "cfg"},
}

// newTestServer builds an MCP server exposing resources, a tool and a prompt
func newTestServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "rclone-test", Title: "Rclone Test Server", Version: "v0.0.1"}, nil)

	resHandler := func(_ context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
		r, ok := testResources[req.Params.URI]
		if !ok {
			return nil, mcp.ResourceNotFoundError(req.Params.URI)
		}
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{{URI: req.Params.URI, Text: r.text}},
		}, nil
	}
	for uri, r := range testResources {
		s.AddResource(&mcp.Resource{URI: uri, Name: path.Base(uri), Size: r.size}, resHandler)
	}

	s.AddTool(&mcp.Tool{
		Name:        "echo",
		Description: "Echo the arguments back",
		InputSchema: json.RawMessage(`{"type":"object"}`),
	}, func(_ context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{
			Content: []mcp.Content{&mcp.TextContent{Text: "echo: " + string(req.Params.Arguments)}},
		}, nil
	})

	s.AddPrompt(&mcp.Prompt{
		Name:        "greet",
		Description: "Greet someone",
		Arguments:   []*mcp.PromptArgument{{Name: "name", Required: true}},
	}, func(_ context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
		return &mcp.GetPromptResult{
			Description: "a greeting",
			Messages: []*mcp.PromptMessage{{
				Role:    "user",
				Content: &mcp.TextContent{Text: "hello " + req.Params.Arguments["name"]},
			}},
		}, nil
	})

	return s
}

// TestMain re-execs the test binary as a stdio MCP server when asked, so the
// stdio transport can be exercised against a real subprocess.
func TestMain(m *testing.M) {
	if os.Getenv(testServerEnv) == "1" {
		if err := newTestServer().Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestResourcePath(t *testing.T) {
	for _, test := range []struct {
		uri  string
		drop bool
		want string
	}{
		{"file:///a.txt", false, "file/a.txt"},
		{"file:///a.txt", true, "a.txt"},
		{"file:///dir/sub/c.txt", false, "file/dir/sub/c.txt"},
		{"file:///dir/sub/c.txt", true, "dir/sub/c.txt"},
		{"file://host/x", false, "file/host/x"},
		{"config://settings", false, "config/settings"},
		{"config://app/settings", true, "app/settings"},
		{"https://example.com/a/b", false, "https/example.com/a/b"},
		{"screen://display1", false, "screen/display1"},
		{"urn:isbn:123", false, "urn/isbn:123"},
		{"file:///%20sp", false, "file/ sp"},
		{"noscheme", false, "noscheme"},
	} {
		assert.Equal(t, test.want, resourcePath(test.uri, test.drop), "%s drop=%v", test.uri, test.drop)
	}
}

func TestSanitizeSegment(t *testing.T) {
	assert.Equal(t, "a_b", sanitizeSegment("a/b"))
	assert.Equal(t, "_.", sanitizeSegment("."))
	assert.Equal(t, "_..", sanitizeSegment(".."))
	assert.Equal(t, "ok", sanitizeSegment("ok"))
}

func TestHeaderPairs(t *testing.T) {
	pairs, err := headerPairs(fs.CommaSepList{"A", "1", "B", "2"})
	require.NoError(t, err)
	assert.Equal(t, [][2]string{{"A", "1"}, {"B", "2"}}, pairs)

	pairs, err = headerPairs(fs.CommaSepList{})
	require.NoError(t, err)
	assert.Empty(t, pairs)

	_, err = headerPairs(fs.CommaSepList{"A"})
	assert.Error(t, err)
}

// listNames returns the sorted base names of the directories and objects in dir
func listNames(t *testing.T, ctx context.Context, f fs.Fs, dir string) (dirs, files []string) {
	t.Helper()
	entries, err := f.List(ctx, dir)
	require.NoError(t, err)
	for _, e := range entries {
		if _, ok := e.(fs.Directory); ok {
			dirs = append(dirs, path.Base(e.Remote()))
		} else {
			files = append(files, path.Base(e.Remote()))
		}
	}
	sort.Strings(dirs)
	sort.Strings(files)
	return dirs, files
}

func readObject(t *testing.T, ctx context.Context, f fs.Fs, remote string, options ...fs.OpenOption) string {
	t.Helper()
	o, err := f.NewObject(ctx, remote)
	require.NoError(t, err)
	rc, err := o.Open(ctx, options...)
	require.NoError(t, err)
	defer func() { _ = rc.Close() }()
	data, err := io.ReadAll(rc)
	require.NoError(t, err)
	return string(data)
}

func mustCommand(t *testing.T, ctx context.Context, f fs.Fs, name string, arg ...string) any {
	t.Helper()
	out, err := f.(fs.Commander).Command(ctx, name, arg, nil)
	require.NoError(t, err)
	return out
}

// testFsBehaviour runs the shared assertions against a connected Fs
func testFsBehaviour(t *testing.T, ctx context.Context, f fs.Fs) {
	// Top level structure
	dirs, files := listNames(t, ctx, f, "")
	assert.Equal(t, []string{"prompts", "resources", "tools"}, dirs)
	assert.Equal(t, []string{"README.md", "config.json", "logs.txt"}, files)

	// Each tool has its own directory with docs, schema and an executable
	dirs, files = listNames(t, ctx, f, "tools")
	assert.Equal(t, []string{"echo"}, dirs)
	assert.Equal(t, []string{"schema.json"}, files)
	_, files = listNames(t, ctx, f, "tools/echo")
	assert.Equal(t, []string{"README.md", "echo", "schema.json"}, files)
	wrapper := readObject(t, ctx, f, "tools/echo/echo")
	assert.Contains(t, wrapper, "#!/usr/bin/env bash")
	assert.Contains(t, wrapper, "rclone backend call mcp-test: echo")

	// Resources are nested by URI scheme
	dirs, files = listNames(t, ctx, f, "resources")
	assert.Equal(t, []string{"config", "file"}, dirs)
	assert.Equal(t, []string{"schema.json"}, files)
	assert.Equal(t, "alpha", readObject(t, ctx, f, "resources/file/a.txt"))
	assert.Equal(t, "charlie", readObject(t, ctx, f, "resources/file/dir/sub/c.txt"))
	// Range read of "alpha" -> bytes 1..3 inclusive == "lph"
	assert.Equal(t, "lph", readObject(t, ctx, f, "resources/file/a.txt", &fs.RangeOption{Start: 1, End: 3}))

	// Prompts
	_, files = listNames(t, ctx, f, "prompts")
	assert.Equal(t, []string{"greet.md", "schema.json"}, files)
	assert.Contains(t, readObject(t, ctx, f, "prompts/greet.md"), "name: greet")

	// Generated docs
	assert.Contains(t, readObject(t, ctx, f, "README.md"), "rclone-test")
	cfg := readObject(t, ctx, f, "config.json")
	assert.Contains(t, cfg, `"tools": true`)
	assert.Contains(t, cfg, `"protocol_version"`)

	// Commands: call a tool with a JSON object
	out := mustCommand(t, ctx, f, "call", "echo", `{"x":1}`).(string)
	assert.Contains(t, out, "echo:")
	assert.Contains(t, out, `"x":1`)

	// Commands: call a tool with key=value pairs and type inference
	out = mustCommand(t, ctx, f, "call", "echo", "text=hi").(string)
	assert.Contains(t, out, `"text":"hi"`)
	out = mustCommand(t, ctx, f, "call", "echo", "n=5", "ok=true").(string)
	assert.Contains(t, out, `"n":5`)
	assert.Contains(t, out, `"ok":true`)

	// Commands: render a prompt
	out = mustCommand(t, ctx, f, "prompt", "greet", "name=bob").(string)
	assert.Contains(t, out, "hello bob")

	// Commands: read a resource by path and by URI
	assert.Equal(t, "alpha", mustCommand(t, ctx, f, "read", "resources/file/a.txt").(string))
	assert.Equal(t, "cfg", mustCommand(t, ctx, f, "read", "config://settings").(string))

	// Commands: listings
	assert.Equal(t, []string{"echo"}, mustCommand(t, ctx, f, "tools").([]string))
	assert.Equal(t, []string{"greet"}, mustCommand(t, ctx, f, "prompts").([]string))
	assert.Contains(t, mustCommand(t, ctx, f, "resources").([]string), "config://settings")

	// Error cases
	_, err := f.NewObject(ctx, "tools")
	assert.Equal(t, fs.ErrorIsDir, err)
	_, err = f.NewObject(ctx, "missing.txt")
	assert.Equal(t, fs.ErrorObjectNotFound, err)
	_, err = f.List(ctx, "does/not/exist")
	assert.Equal(t, fs.ErrorDirNotFound, err)
}

func shutdown(t *testing.T, ctx context.Context, f fs.Fs) {
	t.Helper()
	if do, ok := f.(fs.Shutdowner); ok {
		assert.NoError(t, do.Shutdown(ctx))
	}
}

func TestHTTPTransport(t *testing.T) {
	ctx := context.Background()
	srv := newTestServer()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	ts := httptest.NewServer(handler)
	defer ts.Close()

	f, err := NewFs(ctx, "mcp-test", "", configmap.Simple{
		"transport": transportHTTP,
		"url":       ts.URL,
	})
	require.NoError(t, err)
	defer shutdown(t, ctx, f) // runs before ts.Close (defers are LIFO)

	testFsBehaviour(t, ctx, f)

	// A root that points directly at a file is reported as a file
	fFile, err := NewFs(ctx, "mcp-test", "README.md", configmap.Simple{
		"transport": transportHTTP,
		"url":       ts.URL,
	})
	assert.Equal(t, fs.ErrorIsFile, err)
	if fFile != nil {
		shutdown(t, ctx, fFile)
	}
}

func TestStdioTransport(t *testing.T) {
	exe, err := os.Executable()
	require.NoError(t, err)
	ctx := context.Background()

	// The backend inherits our environment when spawning the command, so
	// setting this makes the spawned test binary act as the MCP server.
	t.Setenv(testServerEnv, "1")

	f, err := NewFs(ctx, "mcp-test", "", configmap.Simple{
		"transport": transportStdio,
		"command":   exe,
	})
	require.NoError(t, err)
	defer shutdown(t, ctx, f)

	testFsBehaviour(t, ctx, f)
}

// newHTTPFs starts an in-process server and returns a connected Fs
func newHTTPFs(t *testing.T, cfg configmap.Simple) (context.Context, fs.Fs) {
	t.Helper()
	ctx := context.Background()
	srv := newTestServer()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv }, nil)
	ts := httptest.NewServer(handler)
	t.Cleanup(ts.Close)
	cfg["transport"] = transportHTTP
	cfg["url"] = ts.URL
	f, err := NewFs(ctx, "mcp-test", "", cfg)
	require.NoError(t, err)
	t.Cleanup(func() { shutdown(t, ctx, f) })
	return ctx, f
}

func TestLogStream(t *testing.T) {
	ctx, f := newHTTPFs(t, configmap.Simple{})

	// Generate some activity so the log has something to show
	mustCommand(t, ctx, f, "call", "echo", "{}")

	// logs.txt is a finite snapshot reflecting activity so far
	logs := readObject(t, ctx, f, "logs.txt")
	assert.Contains(t, logs, "connected to rclone-test")
	assert.Contains(t, logs, "call tool echo")
}

func TestInstall(t *testing.T) {
	ctx, f := newHTTPFs(t, configmap.Simple{})
	dir := t.TempDir()

	out, err := f.(fs.Commander).Command(ctx, "install", []string{dir}, nil)
	require.NoError(t, err)
	res := out.(map[string]any)
	assert.Equal(t, []string{"echo"}, res["tools"])

	script := filepath.Join(dir, "echo")
	info, err := os.Stat(script)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o111, "wrapper should be executable")

	data, err := os.ReadFile(script)
	require.NoError(t, err)
	assert.Contains(t, string(data), "rclone backend call mcp-test: echo")

	// --help runs without needing rclone or the server
	output, err := exec.Command("bash", script, "--help").CombinedOutput()
	require.NoError(t, err, string(output))
	assert.Contains(t, string(output), "Input schema")
	assert.Contains(t, string(output), "echo -")

	// A README index is written too
	assert.FileExists(t, filepath.Join(dir, "README.md"))
}

func TestSocketBridge(t *testing.T) {
	ctx, f := newHTTPFs(t, configmap.Simple{"sockets": "true"})

	// The socket is exposed as a .rclonelink whose content is the real path
	sockPath := readObject(t, ctx, f, "tools/echo/echo.sock"+fs.LinkSuffix)
	require.NotEmpty(t, sockPath)

	conn, err := net.Dial("unix", sockPath)
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()

	_, err = conn.Write([]byte(`{"x":1}`))
	require.NoError(t, err)
	require.NoError(t, conn.(*net.UnixConn).CloseWrite())

	resp, err := io.ReadAll(conn)
	require.NoError(t, err)
	assert.Contains(t, string(resp), "echo:")
	assert.Contains(t, string(resp), `"x":1`)
}
