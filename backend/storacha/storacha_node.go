// Package storacha provides a Storacha backend for rclone using Node.js subprocess
package storacha

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
)

// Fs represents a Storacha filesystem backed by Node.js subprocess
type Fs struct {
	name     string
	root     string
	spaceDID string
	email    string
	node     *NodeBridge
}

// Object represents a file stored in Storacha
type Object struct {
	fs      *Fs
	remote  string
	cid     string
	size    int64
	modTime time.Time
}

// NodeBridge manages communication with Node.js process
type NodeBridge struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Reader
	mu     sync.Mutex
}

// JSRequest is a request to the Node.js process
type JSRequest struct {
	ID     int         `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params"`
}

// JSResponse is a response from the Node.js process
type JSResponse struct {
	ID      int             `json:"id"`
	Success bool            `json:"success"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ------------------------------------------------------------
// Backend registration
// ------------------------------------------------------------

func init() {
	fs.Register(&fs.RegInfo{
		Name:        "storacha",
		Description: "Storacha Decentralized Storage",
		NewFs:       NewFs,
		Options: []fs.Option{
			{
				Name:     "space_did",
				Help:     "Storacha space DID to operate on.",
				Required: true,
			},
			{
				Name:     "email",
				Help:     "Email for Storacha authentication.",
				Required: false,
			},
		},
	})
}

// ------------------------------------------------------------
// Node.js Bridge
// ------------------------------------------------------------

func findNodeScript() (string, error) {
	// Look for the Node.js bridge script
	// Check in the same directory as the executable first
	execPath, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(execPath)
		scriptPath := filepath.Join(dir, "storacha-bridge.mjs")
		if _, err := os.Stat(scriptPath); err == nil {
			return scriptPath, nil
		}
	}

	// Check common locations
	locations := []string{
		"./storacha-bridge.mjs",
		"./backend/storacha/storacha-bridge.mjs",
		"/usr/local/share/rclone/storacha-bridge.mjs",
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			abs, _ := filepath.Abs(loc)
			return abs, nil
		}
	}

	return "", fmt.Errorf("storacha-bridge.mjs not found")
}

func NewNodeBridge(ctx context.Context) (*NodeBridge, error) {
	// Check if Node.js is available
	nodePath, err := exec.LookPath("node")
	if err != nil {
		return nil, fmt.Errorf("Node.js not found. Please install Node.js 18+: %w", err)
	}

	scriptPath, err := findNodeScript()
	if err != nil {
		return nil, err
	}

	cmd := exec.CommandContext(ctx, nodePath, scriptPath)
	cmd.Stderr = os.Stderr // Forward Node.js errors

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start Node.js: %w", err)
	}

	return &NodeBridge{
		cmd:    cmd,
		stdin:  stdin,
		stdout: bufio.NewReader(stdout),
	}, nil
}

func (n *NodeBridge) Call(method string, params interface{}) (*JSResponse, error) {
	n.mu.Lock()
	defer n.mu.Unlock()

	req := JSRequest{
		ID:     1,
		Method: method,
		Params: params,
	}

	// Send request
	reqJSON, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(n.stdin, "%s\n", reqJSON); err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	// Read response
	line, err := n.stdout.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var resp JSResponse
	if err := json.Unmarshal([]byte(line), &resp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	return &resp, nil
}

func (n *NodeBridge) Close() error {
	n.stdin.Close()
	return n.cmd.Wait()
}

// ------------------------------------------------------------
// Fs construction & initialization
// ------------------------------------------------------------

func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	spaceDID, _ := m.Get("space_did")
	email, _ := m.Get("email")

	if spaceDID == "" {
		return nil, fmt.Errorf("storacha: space_did is required")
	}

	// Start Node.js bridge
	node, err := NewNodeBridge(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start Node.js bridge: %w", err)
	}

	f := &Fs{
		name:     name,
		root:     root,
		spaceDID: spaceDID,
		email:    email,
		node:     node,
	}

	// Initialize the client
	resp, err := node.Call("init", map[string]string{
		"spaceDID": spaceDID,
		"email":    email,
	})
	if err != nil {
		node.Close()
		return nil, fmt.Errorf("failed to initialize: %w", err)
	}

	if !resp.Success {
		node.Close()
		return nil, fmt.Errorf("initialization failed: %s", resp.Error)
	}

	return f, nil
}

// ------------------------------------------------------------
// Fs interface
// ------------------------------------------------------------

func (f *Fs) Name() string   { return f.name }
func (f *Fs) Root() string   { return f.root }
func (f *Fs) String() string { return "storacha:" + f.spaceDID }

func (f *Fs) Features() *fs.Features {
	return (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(context.Background(), f)
}

func (f *Fs) Precision() time.Duration {
	return time.Second
}

func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

func (f *Fs) Shutdown(ctx context.Context) error {
	if f.node != nil {
		return f.node.Close()
	}
	return nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	resp, err := f.node.Call("list", map[string]string{
		"path": dir,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("list failed: %s", resp.Error)
	}

	var items []struct {
		Name    string `json:"name"`
		Size    int64  `json:"size"`
		IsDir   bool   `json:"isDir"`
		ModTime string `json:"modTime"`
		CID     string `json:"cid"`
	}

	if err := json.Unmarshal(resp.Result, &items); err != nil {
		return nil, fmt.Errorf("failed to parse list result: %w", err)
	}

	for _, item := range items {
		remote := item.Name
		if dir != "" {
			remote = dir + "/" + item.Name
		}

		if item.IsDir {
			entries = append(entries, fs.NewDir(remote, time.Time{}))
		} else {
			modTime, _ := time.Parse(time.RFC3339, item.ModTime)
			entries = append(entries, &Object{
				fs:      f,
				remote:  remote,
				cid:     item.CID,
				size:    item.Size,
				modTime: modTime,
			})
		}
	}

	return entries, nil
}

// NewObject finds an object by remote path
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	// Query the bridge for file info
	resp, err := f.node.Call("stat", map[string]string{
		"name": remote,
	})
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("stat failed: %s", resp.Error)
	}

	var result struct {
		Found   bool   `json:"found"`
		Name    string `json:"name"`
		CID     string `json:"cid"`
		Size    int64  `json:"size"`
		ModTime string `json:"modTime"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse stat result: %w", err)
	}

	if !result.Found {
		return nil, fs.ErrorObjectNotFound
	}

	modTime, _ := time.Parse(time.RFC3339, result.ModTime)
	return &Object{
		fs:      f,
		remote:  remote,
		cid:     result.CID,
		size:    result.Size,
		modTime: modTime,
	}, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	// Read file data
	data, err := io.ReadAll(in)
	if err != nil {
		return nil, fmt.Errorf("failed to read data: %w", err)
	}

	resp, err := f.node.Call("upload", map[string]interface{}{
		"name": src.Remote(),
		"data": data, // Will be base64 encoded by JSON
		"size": src.Size(),
	})
	if err != nil {
		return nil, fmt.Errorf("upload failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("upload failed: %s", resp.Error)
	}

	var result struct {
		CID string `json:"cid"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse upload result: %w", err)
	}

	return &Object{
		fs:      f,
		remote:  src.Remote(),
		cid:     result.CID,
		size:    src.Size(),
		modTime: src.ModTime(ctx),
	}, nil
}

// Mkdir creates a directory (no-op for Storacha)
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	return nil // Directories are implicit
}

// Rmdir removes a directory (no-op for Storacha)
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return nil
}

// ------------------------------------------------------------
// Object methods
// ------------------------------------------------------------

func (o *Object) Fs() fs.Info                           { return o.fs }
func (o *Object) String() string                        { return o.remote }
func (o *Object) Remote() string                        { return o.remote }
func (o *Object) Size() int64                           { return o.size }
func (o *Object) ModTime(ctx context.Context) time.Time { return o.modTime }
func (o *Object) Storable() bool                        { return true }

func (o *Object) Hash(ctx context.Context, ty hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorNotImplemented
}

func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	if o.cid == "" {
		return nil, fmt.Errorf("cannot open object without CID")
	}

	resp, err := o.fs.node.Call("download", map[string]string{
		"cid": o.cid,
	})
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	if !resp.Success {
		return nil, fmt.Errorf("download failed: %s", resp.Error)
	}

	var result struct {
		Data []byte `json:"data"`
	}
	if err := json.Unmarshal(resp.Result, &result); err != nil {
		return nil, fmt.Errorf("failed to parse download result: %w", err)
	}

	return io.NopCloser(io.NewSectionReader(
		&byteReader{data: result.Data}, 0, int64(len(result.Data)),
	)), nil
}

type byteReader struct {
	data []byte
}

func (b *byteReader) ReadAt(p []byte, off int64) (n int, err error) {
	if off >= int64(len(b.data)) {
		return 0, io.EOF
	}
	n = copy(p, b.data[off:])
	return n, nil
}

func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	_, err := o.fs.Put(ctx, in, src, options...)
	return err
}

func (o *Object) Remove(ctx context.Context) error {
	if o.cid == "" {
		return fmt.Errorf("cannot remove object without CID")
	}

	resp, err := o.fs.node.Call("remove", map[string]string{
		"cid": o.cid,
	})
	if err != nil {
		return fmt.Errorf("remove failed: %w", err)
	}

	if !resp.Success {
		return fmt.Errorf("remove failed: %s", resp.Error)
	}

	return nil
}

// ------------------------------------------------------------
// Interface checks
// ------------------------------------------------------------

var (
	_ fs.Fs         = (*Fs)(nil)
	_ fs.Shutdowner = (*Fs)(nil)
	_ fs.Object     = (*Object)(nil)
)
