// Package proxy implements a programmable proxy for rclone serve
package proxy

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	libcache "github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfscommon"
)

// Help contains text describing how to use the proxy
var Help = strings.ReplaceAll(`### Auth Proxy

If you supply the parameter |--auth-proxy /path/to/program| then
rclone will use that program to generate backends on the fly which
then are used to authenticate incoming requests.  This uses a simple
JSON based protocol with input on STDIN and output on STDOUT.

**PLEASE NOTE:** |--auth-proxy| and |--authorized-keys| cannot be used
together, if |--auth-proxy| is set the authorized keys option will be
ignored.

There is an example program
[bin/test_proxy.py](https://github.com/rclone/rclone/blob/master/bin/test_proxy.py)
in the rclone source code.

The program's job is to take a |user| and |pass| on the input and turn
those into the config for a backend on STDOUT in JSON format.  This
config will have any default parameters for the backend added, but it
won't use configuration from environment variables or command line
options - it is the job of the proxy program to make a complete
config.

This config generated must have this extra parameter
- |_root| - root to use for the backend

And it may have this parameter
- |_obscure| - comma separated strings for parameters to obscure

If password authentication was used by the client, input to the proxy
process (on STDIN) would look similar to this:

|||
{
	"user": "me",
	"pass": "mypassword"
}
|||

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

|||
{
	"user": "me",
	"public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf"
}
|||

And as an example return this on STDOUT

|||
{
	"type": "sftp",
	"_root": "",
	"_obscure": "pass",
	"user": "me",
	"pass": "mypassword",
	"host": "sftp.example.com"
}
|||

This would mean that an SFTP backend would be created on the fly for
the |user| and |pass|/|public_key| returned in the output to the host given.  Note
that since |_obscure| is set to |pass|, rclone will obscure the |pass|
parameter before creating the backend (which is required for sftp
backends).

The program can manipulate the supplied |user| in any way, for example
to make proxy to many different sftp backends, you could make the
|user| be |user@example.com| and then set the |host| to |example.com|
in the output and the user to |user|. For security you'd probably want
to restrict the |host| to a limited list.

Note that an internal cache is keyed on |user| so only use that for
configuration, don't use |pass| or |public_key|.  This also means that if a user's
password or public-key is changed the cache will need to expire (which takes 5 mins)
before it takes effect.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.  

`, "|", "`")

// Options is options for creating the proxy
type Options struct {
	AuthProxy string
}

// DefaultOpt is the default values uses for Opt
var DefaultOpt = Options{
	AuthProxy: "",
}

// Proxy represents a proxy to turn auth requests into a VFS
type Proxy struct {
	cmdLine  []string // broken down command line
	vfsCache *libcache.Cache
	ctx      context.Context // for global config
	Opt      Options
}

// cacheEntry is what is stored in the vfsCache
type cacheEntry struct {
	vfs    *vfs.VFS          // stored VFS
	pwHash [sha256.Size]byte // sha256 hash of the password/publicKey
}

// New creates a new proxy with the Options passed in
func New(ctx context.Context, opt *Options) *Proxy {
	return &Proxy{
		ctx:      ctx,
		Opt:      *opt,
		cmdLine:  strings.Fields(opt.AuthProxy),
		vfsCache: libcache.New(),
	}
}

// run the proxy command returning a config map
func (p *Proxy) run(in map[string]string) (config configmap.Simple, err error) {
	cmd := exec.Command(p.cmdLine[0], p.cmdLine[1:]...)
	inBytes, err := json.MarshalIndent(in, "", "\t")
	if err != nil {
		return nil, fmt.Errorf("proxy: failed to marshal input: %w", err)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdin = bytes.NewBuffer(inBytes)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	start := time.Now()
	err = cmd.Run()
	fs.Debugf(nil, "Calling proxy %v", p.cmdLine)
	duration := time.Since(start)
	if err != nil {
		return nil, fmt.Errorf("proxy: failed on %v: %q: %w", p.cmdLine, strings.TrimSpace(stderr.String()), err)
	}
	err = json.Unmarshal(stdout.Bytes(), &config)
	if err != nil {
		return nil, fmt.Errorf("proxy: failed to read output: %q: %w", stdout.String(), err)
	}
	fs.Debugf(nil, "Proxy returned in %v", duration)

	// Obscure any values in the config map that need it
	obscureFields, ok := config.Get("_obscure")
	if ok {
		for _, key := range strings.Split(obscureFields, ",") {
			value, ok := config.Get(key)
			if ok {
				obscuredValue, err := obscure.Obscure(value)
				if err != nil {
					return nil, fmt.Errorf("proxy: %w", err)
				}
				config.Set(key, obscuredValue)
			}
		}
	}
	return config, nil
}

// call runs the auth proxy and returns a cacheEntry and an error
func (p *Proxy) call(user, auth string, isPublicKey bool) (value interface{}, err error) {
	var config configmap.Simple
	// Contact the proxy
	if isPublicKey {
		config, err = p.run(map[string]string{
			"user":       user,
			"public_key": auth,
		})
	} else {
		config, err = p.run(map[string]string{
			"user": user,
			"pass": auth,
		})
	}

	if err != nil {
		return nil, err
	}

	// Look for required fields in the answer
	fsName, ok := config.Get("type")
	if !ok {
		return nil, errors.New("proxy: type not set in result")
	}
	root, ok := config.Get("_root")
	if !ok {
		return nil, errors.New("proxy: _root not set in result")
	}

	// Find the backend
	fsInfo, err := fs.Find(fsName)
	if err != nil {
		return nil, fmt.Errorf("proxy: couldn't find backend for %q: %w", fsName, err)
	}

	// base name of config on user name.  This may appear in logs
	name := "proxy-" + user
	fsString := name + ":" + root

	// Look for fs in the VFS cache
	value, err = p.vfsCache.Get(user, func(key string) (value interface{}, ok bool, err error) {
		// Create the Fs from the cache
		f, err := cache.GetFn(p.ctx, fsString, func(ctx context.Context, fsString string) (fs.Fs, error) {
			// Update the config with the default values
			for i := range fsInfo.Options {
				o := &fsInfo.Options[i]
				if _, found := config.Get(o.Name); !found && o.Default != nil && o.String() != "" {
					config.Set(o.Name, o.String())
				}
			}
			return fsInfo.NewFs(ctx, name, root, config)
		})
		if err != nil {
			return nil, false, err
		}

		// We hash the auth here so we don't copy the auth more than we
		// need to in memory. An attacker would find it easier to go
		// after the unencrypted password in memory most likely.
		entry := cacheEntry{
			vfs:    vfs.New(f, &vfscommon.Opt),
			pwHash: sha256.Sum256([]byte(auth)),
		}
		return entry, true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("proxy: failed to create backend: %w", err)
	}
	return value, nil
}

// Call runs the auth proxy with the username and password/public key provided
// returning a *vfs.VFS and the key used in the VFS cache.
func (p *Proxy) Call(user, auth string, isPublicKey bool) (VFS *vfs.VFS, vfsKey string, err error) {
	// Look in the cache first
	value, ok := p.vfsCache.GetMaybe(user)

	// If not found then call the proxy for a fresh answer
	if !ok {
		value, err = p.call(user, auth, isPublicKey)
		if err != nil {
			return nil, "", err
		}
	}

	// check we got what we were expecting
	entry, ok := value.(cacheEntry)
	if !ok {
		return nil, "", fmt.Errorf("proxy: value is not cache entry: %#v", value)
	}

	// Check the password / public key is correct in the cached entry.  This
	// prevents an attack where subsequent requests for the same
	// user don't have their auth checked. It does mean that if
	// the password is changed, the user will have to wait for
	// cache expiry (5m) before trying again.
	authHash := sha256.Sum256([]byte(auth))
	if subtle.ConstantTimeCompare(authHash[:], entry.pwHash[:]) != 1 {
		if isPublicKey {
			return nil, "", errors.New("proxy: incorrect public key")
		}
		return nil, "", errors.New("proxy: incorrect password")
	}

	return entry.vfs, user, nil
}

// Get VFS from the cache using key - returns nil if not found
func (p *Proxy) Get(key string) *vfs.VFS {
	value, ok := p.vfsCache.GetMaybe(key)
	if !ok {
		return nil
	}
	entry := value.(cacheEntry)
	return entry.vfs
}
