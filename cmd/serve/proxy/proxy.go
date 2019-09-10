// Package proxy implements a programmable proxy for rclone serve
package proxy

import (
	"bytes"
	"encoding/json"
	"os/exec"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	libcache "github.com/rclone/rclone/lib/cache"
	"github.com/rclone/rclone/vfs"
	"github.com/rclone/rclone/vfs/vfsflags"
	"golang.org/x/crypto/bcrypt"
)

// Help contains text describing how to use the proxy
var Help = strings.Replace(`
### Auth Proxy

If you supply the parameter |--auth-proxy /path/to/program| then
rclone will use that program to generate backends on the fly which
then are used to authenticate incoming requests.  This uses a simple
JSON based protocl with input on STDIN and output on STDOUT.

There is an example program
[bin/test_proxy.py](https://github.com/rclone/rclone/blob/master/test_proxy.py)
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

For example the program might take this on STDIN

|||
{
	"user": "me",
	"pass": "mypassword"
}
|||

And return this on STDOUT

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
the |user| and |pass| returned in the output to the host given.  Note
that since |_obscure| is set to |pass|, rclone will obscure the |pass|
parameter before creating the backend (which is required for sftp
backends).

The progam can manipulate the supplied |user| in any way, for example
to make proxy to many different sftp backends, you could make the
|user| be |user@example.com| and then set the |host| to |example.com|
in the output and the user to |user|. For security you'd probably want
to restrict the |host| to a limited list.

Note that an internal cache is keyed on |user| so only use that for
configuration, don't use |pass|.  This also means that if a user's
password is changed the cache will need to expire (which takes 5 mins)
before it takes effect.

This can be used to build general purpose proxies to any kind of
backend that rclone supports.  
`, "|", "`", -1)

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
	Opt      Options
}

// cacheEntry is what is stored in the vfsCache
type cacheEntry struct {
	vfs    *vfs.VFS // stored VFS
	pwHash []byte   // bcrypt hash of the password
}

// New creates a new proxy with the Options passed in
func New(opt *Options) *Proxy {
	return &Proxy{
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
		return nil, errors.Wrap(err, "Proxy.Call failed to marshal input: %v")
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
		return nil, errors.Wrapf(err, "proxy: failed on %v: %q", p.cmdLine, strings.TrimSpace(string(stderr.Bytes())))
	}
	err = json.Unmarshal(stdout.Bytes(), &config)
	if err != nil {
		return nil, errors.Wrapf(err, "proxy: failed to read output: %q", string(stdout.Bytes()))
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
					return nil, errors.Wrap(err, "proxy")
				}
				config.Set(key, obscuredValue)
			}
		}
	}
	return config, nil
}

// call runs the auth proxy and returns a cacheEntry and an error
func (p *Proxy) call(user, pass string, passwordBytes []byte) (value interface{}, err error) {
	// Contact the proxy
	config, err := p.run(map[string]string{
		"user": user,
		"pass": pass,
	})
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
		return nil, errors.Wrapf(err, "proxy: couldn't find backend for %q", fsName)
	}

	// base name of config on user name.  This may appear in logs
	name := "proxy-" + user
	fsString := name + ":" + root

	// Look for fs in the VFS cache
	value, err = p.vfsCache.Get(user, func(key string) (value interface{}, ok bool, err error) {
		// Create the Fs from the cache
		f, err := cache.GetFn(fsString, func(fsString string) (fs.Fs, error) {
			// Update the config with the default values
			for i := range fsInfo.Options {
				o := &fsInfo.Options[i]
				if _, found := config.Get(o.Name); !found && o.Default != nil && o.String() != "" {
					config.Set(o.Name, o.String())
				}
			}
			return fsInfo.NewFs(name, root, config)
		})
		if err != nil {
			return nil, false, err
		}
		pwHash, err := bcrypt.GenerateFromPassword(passwordBytes, bcrypt.DefaultCost)
		if err != nil {
			return nil, false, err
		}
		entry := cacheEntry{
			vfs:    vfs.New(f, &vfsflags.Opt),
			pwHash: pwHash,
		}
		return entry, true, nil
	})
	if err != nil {
		return nil, errors.Wrapf(err, "proxy: failed to create backend")
	}
	return value, nil
}

// Call runs the auth proxy with the given input, returning a *vfs.VFS
// and the key used in the VFS cache.
func (p *Proxy) Call(user, pass string) (VFS *vfs.VFS, vfsKey string, err error) {
	var passwordBytes = []byte(pass)

	// Look in the cache first
	value, ok := p.vfsCache.GetMaybe(user)

	// If not found then call the proxy for a fresh answer
	if !ok {
		value, err = p.call(user, pass, passwordBytes)
		if err != nil {
			return nil, "", err
		}
	}

	// check we got what we were expecting
	entry, ok := value.(cacheEntry)
	if !ok {
		return nil, "", errors.Errorf("proxy: value is not cache entry: %#v", value)
	}

	// Check the password is correct in the cached entry.  This
	// prevents an attack where subsequent requests for the same
	// user don't have their auth checked. It does mean that if
	// the password is changed, the user will have to wait for
	// cache expiry (5m) before trying again.
	err = bcrypt.CompareHashAndPassword(entry.pwHash, passwordBytes)
	if err != nil {
		return nil, "", errors.Wrap(err, "proxy: incorrect password")
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
