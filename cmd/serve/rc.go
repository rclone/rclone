package serve

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"net"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/filter"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/lib/errcount"
)

// Handle describes what a server can do
type Handle interface {
	// Addr returns the listening address of the server
	Addr() net.Addr

	// Shutdown stops the server
	Shutdown() error

	// Serve starts the server - doesn't return until Shutdown is called.
	Serve() (err error)
}

// Describes a running server
type server struct {
	ID      string     `json:"id"`     // id of the server
	Addr    string     `json:"addr"`   // address of the server
	Params  rc.Params  `json:"params"` // Parameters used to start the server
	h       Handle     `json:"-"`      // control the server
	errChan chan error `json:"-"`      // receive errors from the server process
}

// Fn starts an rclone serve command
type Fn func(ctx context.Context, f fs.Fs, in rc.Params) (Handle, error)

// Globals
var (
	// mutex to protect all the variables in this block
	serveMu sync.Mutex
	// Serve functions available
	serveFns = map[string]Fn{}
	// Running servers
	servers = map[string]*server{}
)

// AddRc adds the named serve function to the rc
func AddRc(name string, serveFunction Fn) {
	serveMu.Lock()
	defer serveMu.Unlock()
	serveFns[name] = serveFunction
}

// unquote `
func q(s string) string {
	return strings.ReplaceAll(s, "|", "`")
}

func init() {
	rc.Add(rc.Call{
		Path:         "serve/start",
		AuthRequired: true,
		Fn:           startRc,
		Title:        "Create a new server",
		Help: q(`Create a new server with the specified parameters.

This takes the following parameters:

- |type| - type of server: |http|, |webdav|, |ftp|, |sftp|, |nfs|, etc.
- |fs| - remote storage path to serve
- |addr| - the ip:port to run the server on, eg ":1234" or "localhost:1234"

Other parameters are as described in the documentation for the
relevant [rclone serve](/commands/rclone_serve/) command line options.
To translate a command line option to an rc parameter, remove the
leading |--| and replace |-| with |_|, so |--vfs-cache-mode| becomes
|vfs_cache_mode|. Note that global parameters must be set with
|_config| and |_filter| as described above.

Examples:

    rclone rc serve/start type=nfs fs=remote: addr=:4321 vfs_cache_mode=full
    rclone rc serve/start --json '{"type":"nfs","fs":"remote:","addr":":1234","vfs_cache_mode":"full"}'

This will give the reply

|||json
{
    "addr": "[::]:4321", // Address the server was started on
    "id": "nfs-ecfc6852" // Unique identifier for the server instance
}
|||

Or an error if it failed to start.

Stop the server with |serve/stop| and list the running servers with |serve/list|.
`),
	})
}

// startRc allows the serve command to be run from rc
func startRc(ctx context.Context, in rc.Params) (out rc.Params, err error) {
	serveType, err := in.GetString("type")

	serveMu.Lock()
	defer serveMu.Unlock()

	serveFn := serveFns[serveType]
	if serveFn == nil {
		return nil, fmt.Errorf("could not find serve type=%q", serveType)
	}

	// Get Fs.fs to be served from fs parameter in the params
	f, err := rc.GetFs(ctx, in)
	if err != nil {
		return nil, err
	}

	// Make a background context and copy the config back.
	newCtx := context.Background()
	newCtx = fs.CopyConfig(newCtx, ctx)
	newCtx = filter.CopyConfig(newCtx, ctx)

	// Start the server
	h, err := serveFn(newCtx, f, in)
	if err != nil {
		return nil, fmt.Errorf("could not start serve %q: %w", serveType, err)
	}

	// Start the server running in the background
	errChan := make(chan error, 1)
	go func() {
		errChan <- h.Serve()
		close(errChan)
	}()

	// Wait for a short length of time to see if an error occurred
	select {
	case err = <-errChan:
		if err == nil {
			err = errors.New("server stopped immediately")
		}
	case <-time.After(100 * time.Millisecond):
		err = nil
	}
	if err != nil {
		return nil, fmt.Errorf("error when starting serve %q: %w", serveType, err)
	}

	// Store it for later
	runningServer := server{
		ID:      fmt.Sprintf("%s-%08x", serveType, rand.Uint32()),
		Params:  in,
		Addr:    h.Addr().String(),
		h:       h,
		errChan: errChan,
	}
	servers[runningServer.ID] = &runningServer

	out = rc.Params{
		"id":   runningServer.ID,
		"addr": runningServer.Addr,
	}

	fs.Debugf(f, "Started serve %s on %s", serveType, runningServer.Addr)
	return out, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "serve/stop",
		AuthRequired: true,
		Fn:           stopRc,
		Title:        "Unserve selected active serve",
		Help: q(`Stops a running |serve| instance by ID.

This takes the following parameters:

- id: as returned by serve/start

This will give an empty response if successful or an error if not.

Example:

    rclone rc serve/stop id=12345
`),
	})
}

// stopRc stops the server process
func stopRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	id, err := in.GetString("id")
	if err != nil {
		return nil, err
	}
	serveMu.Lock()
	defer serveMu.Unlock()
	s := servers[id]
	if s == nil {
		return nil, fmt.Errorf("server with id=%q not found", id)
	}
	err = s.h.Shutdown()
	<-s.errChan // ignore server return error - likely is "use of closed network connection"
	delete(servers, id)
	return nil, err
}

func init() {
	rc.Add(rc.Call{
		Path:         "serve/types",
		AuthRequired: true,
		Fn:           serveTypesRc,
		Title:        "Show all possible serve types",
		Help: q(`This shows all possible serve types and returns them as a list.

This takes no parameters and returns

- types: list of serve types, eg "nfs", "sftp", etc

The serve types are strings like "serve", "serve2", "cserve" and can
be passed to serve/start as the serveType parameter.

Eg

    rclone rc serve/types

Returns

|||json
{
    "types": [
        "http",
        "sftp",
        "nfs"
    ]
}
|||
`),
	})
}

// serveTypesRc returns a list of available serve types.
func serveTypesRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	var serveTypes = []string{}
	serveMu.Lock()
	defer serveMu.Unlock()
	for serveType := range serveFns {
		serveTypes = append(serveTypes, serveType)
	}
	sort.Strings(serveTypes)
	return rc.Params{
		"types": serveTypes,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "serve/list",
		AuthRequired: true,
		Fn:           listRc,
		Title:        "Show running servers",
		Help: q(`Show running servers with IDs.

This takes no parameters and returns

- list: list of running serve commands

Each list element will have

- id: ID of the server
- addr: address the server is running on
- params: parameters used to start the server

Eg

    rclone rc serve/list

Returns

|||json
{
    "list": [
        {
            "addr": "[::]:4321",
            "id": "nfs-ffc2a4e5",
            "params": {
                "fs": "remote:",
                "opt": {
                    "ListenAddr": ":4321"
                },
                "type": "nfs",
                "vfsOpt": {
                    "CacheMode": "full"
                }
            }
        }
    ]
}
|||
`),
	})
}

// listRc returns a list of current serves sorted by serve path
func listRc(_ context.Context, in rc.Params) (out rc.Params, err error) {
	serveMu.Lock()
	defer serveMu.Unlock()
	list := []*server{}
	for _, item := range servers {
		list = append(list, item)
	}
	slices.SortFunc(list, func(a, b *server) int {
		return cmp.Compare(a.ID, b.ID)
	})
	return rc.Params{
		"list": list,
	}, nil
}

func init() {
	rc.Add(rc.Call{
		Path:         "serve/stopall",
		AuthRequired: true,
		Fn:           stopAll,
		Title:        "Stop all active servers",
		Help: q(`Stop all active servers.

This will stop all active servers.

    rclone rc serve/stopall
`),
	})
}

// stopAll shuts all the servers down
func stopAll(_ context.Context, in rc.Params) (out rc.Params, err error) {
	serveMu.Lock()
	defer serveMu.Unlock()
	ec := errcount.New()
	for id, s := range servers {
		ec.Add(s.h.Shutdown())
		<-s.errChan // ignore server return error - likely is "use of closed network connection"
		delete(servers, id)
	}
	return nil, ec.Err("error when stopping server")
}
