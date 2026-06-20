// Package proxy implements a programmable proxy for rclone serve
package proxy

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
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

|||json
{
  "user": "me",
  "pass": "mypassword"
}
|||

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

|||json
{
  "user": "me",
  "public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf"
}
|||

And as an example return this on STDOUT

|||json
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

An internal cache of backends is keyed on the |user| and a hash of the
|pass| or |public_key|.  This means that if a user's password or
public-key changes, or the proxy returns different config parameters
(eg a rotated |api_key|), a fresh backend will be created on the next
request rather than the cached one being reused.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.

`, "|", "`")

// OptionsInfo descripts the Options in use
var OptionsInfo = fs.Options{{
	Name:    "auth_proxy",
	Default: "",
	Help:    "A program to use to create the backend from the auth",
}}

// Options is options for creating the proxy
type Options struct {
	AuthProxy string `config:"auth_proxy"`
}

// Opt is the default options
var Opt Options

func init() {
	fs.RegisterGlobalOptions(fs.OptionsInfo{Name: "proxy", Opt: &Opt, Options: OptionsInfo})
}

// Proxy represents a proxy to turn auth requests into a VFS
type Proxy struct {
	cmdLine  []string // broken down command line
	vfsCache *libcache.Cache
	ctx      context.Context // for global config
	Opt      Options
	vfsOpt   vfscommon.Options
}

// cacheEntry is what is stored in the vfsCache
type cacheEntry struct {
	vfs    *vfs.VFS          // stored VFS
	pwHash [sha256.Size]byte // sha256 hash of the password/publicKey
}

// New creates a new proxy with the Options passed in
//
// Any VFS are created with the vfsOpt passed in.
func New(ctx context.Context, opt *Options, vfsOpt *vfscommon.Options) *Proxy {
	return &Proxy{
		ctx:      ctx,
		Opt:      *opt,
		cmdLine:  strings.Fields(opt.AuthProxy),
		vfsCache: libcache.New(),
		vfsOpt:   *vfsOpt,
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
		for key := range strings.SplitSeq(obscureFields, ",") {
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

// cacheKeyHMACKey is a per-process random key used to derive cache keys
// from auth credentials. Using a keyed hash (HMAC) rather than a bare
// SHA256 means the hash fragment that appears in logs and backend names
// cannot be used to brute-force the underlying password offline.
var cacheKeyHMACKey = func() []byte {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		panic(fmt.Sprintf("proxy: failed to generate cache key: %v", err))
	}
	return key
}()

// generateCacheKey creates a composite cache key from user and auth credentials
func generateCacheKey(user, auth string) string {
	mac := hmac.New(sha256.New, cacheKeyHMACKey)
	mac.Write([]byte(auth))
	return user + "-" + hex.EncodeToString(mac.Sum(nil)[:8])
}

// call runs the auth proxy and returns a cacheEntry and an error
func (p *Proxy) call(user, auth string, isPublicKey bool) (value any, err error) {
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

	// Make the cache key include the auth so that changes to the
	// auth (eg the proxy returning new config) create a fresh
	// backend rather than reusing the cached one.
	cacheKey := generateCacheKey(user, auth)

	// base name of config on user name and auth hash.  This may appear in logs
	name := "proxy-" + cacheKey
	fsString := name + ":" + root

	// Look for fs in the VFS cache
	value, err = p.vfsCache.Get(cacheKey, func(key string) (value any, ok bool, err error) {
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
			vfs:    vfs.New(p.ctx, f, &p.vfsOpt),
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
	// Cache key includes the auth so credential changes don't hit a stale entry
	cacheKey := generateCacheKey(user, auth)

	// Look in the cache first with the credential-aware key
	value, ok := p.vfsCache.GetMaybe(cacheKey)

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

	// Check the password / public key matches the cached entry. The
	// cache key already includes a hash of the auth, so a changed
	// credential lands on a fresh key rather than this entry; this
	// check is a final guard against a hash collision on the key
	// returning a backend created with different auth.
	authHash := sha256.Sum256([]byte(auth))
	if subtle.ConstantTimeCompare(authHash[:], entry.pwHash[:]) != 1 {
		if isPublicKey {
			return nil, "", errors.New("proxy: incorrect public key")
		}
		return nil, "", errors.New("proxy: incorrect password")
	}

	return entry.vfs, cacheKey, nil
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
