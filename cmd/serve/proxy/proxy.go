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
	"net/netip"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/cache"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	libcache "github.com/rclone/rclone/lib/cache"
	libhttp "github.com/rclone/rclone/lib/http"
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

And it may have these parameters

- |_obscure| - comma separated strings for parameters to obscure
- |_secret_access_key| - the secret for S3 access key auth (see below)

If password authentication was used by the client, input to the proxy
process (on STDIN) would look similar to this:

|||json
{
  "user": "me",
  "pass": "mypassword",
  "client_ip": "192.168.1.1"
}
|||

If public-key authentication was used by the client, input to the
proxy process (on STDIN) would look similar to this:

|||json
{
  "user": "me",
  "public_key": "AAAAB3NzaC1yc2EAAAADAQABAAABAQDuwESFdAe14hVS6omeyX7edc...JQdf",
  "client_ip": "192.168.1.1"
}
|||

If the client authenticated with an S3 access key (|rclone serve s3|),
the client never sends its secret, only a signature made with it, so
the input contains just the access key ID as the |user| with no |pass|
or |public_key|:

|||json
{
  "user": "AKIAIOSFODNN7EXAMPLE",
  "client_ip": "192.168.1.1"
}
|||

In this case the program must look up the secret access key for that
access key ID and return it in the |_secret_access_key| field of the
output. Rclone then uses that secret to verify the signature on the
request, refusing the request if it does not match. This means the
proxy program is the source of truth for both the credentials and the
backend they map to. If the program does not return
|_secret_access_key| or returns it empty the request is refused.

The program's answer for an access key ID is cached (see below) but
is checked with the program again after 5 minutes even if the access
key ID is in constant use, so revoking an access key ID in the
program takes effect within 5 minutes. A rotated secret takes effect
on the first request signed with it.

The |client_ip| key holds the IP address the client connected from,
without a port number.  It can be used to restrict logins to certain
networks, or to log authentication attempts centrally.  It is omitted if
the client has no IP address, for example when connecting over a unix
socket.  Note that if rclone is behind a reverse proxy this will be the
address of the reverse proxy and not the original client.

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

An internal cache of backends is keyed on the |user|, a hash of the
|pass| or |public_key|, and the |client_ip|.  This means that if a
user's password or public-key changes, the client connects from a new IP
address, or the proxy returns different config parameters (eg a rotated
|api_key|), a fresh backend will be created on the next request rather
than the cached one being reused.

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
	cmdLine     []string // broken down command line
	vfsCache    *libcache.Cache
	ctx         context.Context // for global config
	Opt         Options
	vfsOpt      vfscommon.Options
	accessKeyMu sync.Mutex // serialises replacing a cached access key entry
}

// cacheEntry is what is stored in the vfsCache
type cacheEntry struct {
	vfs       *vfs.VFS          // stored VFS
	pwHash    [sha256.Size]byte // sha256 hash of the password/publicKey
	secret    string            // secret access key returned by the proxy for access key auth
	refreshed *atomic.Int64     // unix nanoseconds when the proxy last confirmed this entry
}

// accessKeyRefreshInterval is the minimum time between consulting the
// proxy again for a cached access key ID whose secret failed to
// verify a request, so a stream of bad signatures for a known access
// key ID can't make the proxy run for every request.
//
// A variable so tests can adjust it.
var accessKeyRefreshInterval = 10 * time.Second

// accessKeyRevalidateInterval is how long a cached access key ID is
// trusted before the proxy is asked about it again even though its
// signatures are still verifying, so a revoked access key ID stops
// working within this time rather than for as long as it stays in
// use.
//
// A variable so tests can adjust it.
var accessKeyRevalidateInterval = 5 * time.Minute

// authKind is the kind of credential the client presented
type authKind int

const (
	authPassword  authKind = iota // auth is a password
	authPublicKey                 // auth is a public key
	authAccessKey                 // user is an S3 access key ID and auth is empty
)

// New creates a new proxy with the Options passed in
//
// Any VFS are created with the vfsOpt passed in.
func New(ctx context.Context, opt *Options, vfsOpt *vfscommon.Options) *Proxy {
	p := &Proxy{
		ctx:      ctx,
		Opt:      *opt,
		cmdLine:  strings.Fields(opt.AuthProxy),
		vfsCache: libcache.New(),
		vfsOpt:   *vfsOpt,
	}
	p.vfsCache.SetFinalizer(func(value any) {
		if entry, ok := value.(cacheEntry); ok && entry.vfs != nil {
			entry.vfs.Shutdown()
		}
	})
	return p
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

// ipFromAddr returns the bare IP from a "host:port" address, or "" if it has none.
func ipFromAddr(addr string) string {
	ap, err := netip.ParseAddrPort(addr)
	if err != nil {
		return ""
	}
	return ap.Addr().Unmap().String()
}

// generateCacheKey creates a composite cache key from the user, the auth
// credentials and the client's IP address.
func generateCacheKey(user, auth, clientIP string) string {
	mac := hmac.New(sha256.New, cacheKeyHMACKey)
	mac.Write([]byte(auth))
	// Separate the two so ("ab", "c") can't collide with ("a", "bc")
	mac.Write([]byte{0})
	mac.Write([]byte(clientIP))
	return user + "-" + hex.EncodeToString(mac.Sum(nil)[:8])
}

// call runs the auth proxy and returns a cacheEntry and an error
func (p *Proxy) call(user, auth string, kind authKind, clientIP string) (value any, err error) {
	// Contact the proxy
	in := map[string]string{
		"user": user,
	}
	switch kind {
	case authPublicKey:
		in["public_key"] = auth
	case authPassword:
		in["pass"] = auth
	}
	if clientIP != "" {
		in["client_ip"] = clientIP
	}
	config, err := p.run(in)
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
	var secret string
	if kind == authAccessKey {
		secret, ok = config.Get("_secret_access_key")
		if !ok {
			return nil, errors.New("proxy: _secret_access_key not set in result")
		}
		// An empty secret would let anyone sign for this access key ID
		if secret == "" {
			return nil, errors.New("proxy: _secret_access_key is empty in result")
		}
		// Keep the secret out of the backend config
		delete(config, "_secret_access_key")
	}

	// Find the backend
	fsInfo, err := fs.Find(fsName)
	if err != nil {
		return nil, fmt.Errorf("proxy: couldn't find backend for %q: %w", fsName, err)
	}

	// Make the cache key include the auth and the client IP so that
	// changes to either (eg the proxy returning new config) create a
	// fresh backend rather than reusing the cached one.
	cacheKey := generateCacheKey(user, auth, clientIP)

	// For access key auth the secret isn't part of the cache key as
	// it isn't known until the proxy has been run, so retire any
	// cached entry whose secret the proxy has since changed.
	//
	// The stale entry is moved to a key including its old secret
	// rather than deleted so that its VFS isn't shut down under
	// requests still using it - it expires with the rest of the cache
	// once it has been unused for long enough.
	//
	// The check, rename and the creation below are done under a lock
	// so that two concurrent refreshes during a rotation can't have
	// the second retire the backend the first has just created.
	if kind == authAccessKey {
		p.accessKeyMu.Lock()
		defer p.accessKeyMu.Unlock()
		if old, ok := p.vfsCache.GetMaybe(cacheKey); ok {
			if entry, ok := old.(cacheEntry); ok && entry.secret != secret {
				p.vfsCache.Rename(cacheKey, generateCacheKey(user, entry.secret, clientIP))
			}
		}
	}

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
			vfs:       vfs.New(p.ctx, f, &p.vfsOpt),
			pwHash:    sha256.Sum256([]byte(auth)),
			secret:    secret,
			refreshed: new(atomic.Int64),
		}
		return entry, true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("proxy: failed to create backend: %w", err)
	}
	// The proxy has just confirmed this entry whether it was created
	// or reused
	if entry, ok := value.(cacheEntry); ok {
		entry.refreshed.Store(time.Now().UnixNano())
	}
	return value, nil
}

// Call runs the auth proxy with the username and password/public key provided
// returning a *vfs.VFS and the key used in the VFS cache.
//
// remoteAddr is the address of the client as returned by net.Addr.String().
// It may be empty if the client has no IP address, for example when
// connecting over a unix socket.
func (p *Proxy) Call(user, auth string, isPublicKey bool, remoteAddr string) (VFS *vfs.VFS, vfsKey string, err error) {
	clientIP := ipFromAddr(remoteAddr)

	// Cache key includes the auth and the client IP so credential or
	// address changes don't hit a stale entry
	cacheKey := generateCacheKey(user, auth, clientIP)

	// Look in the cache first with the credential-aware key
	value, ok := p.vfsCache.GetMaybe(cacheKey)

	// If not found then call the proxy for a fresh answer
	if !ok {
		kind := authPassword
		if isPublicKey {
			kind = authPublicKey
		}
		value, err = p.call(user, auth, kind, clientIP)
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

// CallAccessKey runs the auth proxy for an S3 access key ID returning
// a *vfs.VFS and the secret access key the proxy supplied for it.
//
// The caller must verify the request's signature against the returned
// secret - the proxy only maps the access key ID to a backend and
// secret, it cannot authenticate the client itself.
//
// If refresh is true the proxy is consulted rather than using a cached
// answer, unless it was consulted for this access key ID less than
// accessKeyRefreshInterval ago. A refresh never shuts down a cached
// backend, even if the proxy returns a different secret, so this is
// safe to do when a signature fails to verify in case the secret has
// been rotated.
//
// A cached answer older than accessKeyRevalidateInterval is always
// checked with the proxy so a revoked access key ID is refused even
// if it is in constant use.
//
// remoteAddr is the address of the client as returned by net.Addr.String().
func (p *Proxy) CallAccessKey(accessKeyID, remoteAddr string, refresh bool) (VFS *vfs.VFS, secret string, err error) {
	clientIP := ipFromAddr(remoteAddr)
	cacheKey := generateCacheKey(accessKeyID, "", clientIP)
	value, ok := p.vfsCache.GetMaybe(cacheKey)
	if ok {
		if entry, isEntry := value.(cacheEntry); isEntry {
			age := time.Since(time.Unix(0, entry.refreshed.Load()))
			switch {
			case age >= accessKeyRevalidateInterval:
				refresh = true
			case age < accessKeyRefreshInterval:
				refresh = false
			}
		}
	}
	if !ok || refresh {
		value, err = p.call(accessKeyID, "", authAccessKey, clientIP)
		if err != nil {
			return nil, "", err
		}
	}
	entry, ok := value.(cacheEntry)
	if !ok {
		return nil, "", fmt.Errorf("proxy: value is not cache entry: %#v", value)
	}
	return entry.vfs, entry.secret, nil
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

// Shutdown shuts down all cached VFS instances
func (p *Proxy) Shutdown() {
	if p != nil && p.vfsCache != nil {
		p.vfsCache.Clear()
	}
}

// Provider hands out VFS instances, either a fixed one or per-user via an auth proxy.
type Provider struct {
	vfs   *vfs.VFS // set if not using an auth proxy
	proxy *Proxy   // set if using an auth proxy
}

// NewProvider creates a Provider. If proxyOpt.AuthProxy is set it creates an
// auth proxy; otherwise it creates a fixed VFS from f and vfsOpt.
func NewProvider(ctx context.Context, f fs.Fs, vfsOpt *vfscommon.Options, proxyOpt *Options) *Provider {
	p := &Provider{}
	if proxyOpt != nil && proxyOpt.AuthProxy != "" {
		p.proxy = New(ctx, proxyOpt, vfsOpt)
	} else {
		p.vfs = vfs.New(ctx, f, vfsOpt)
	}
	return p
}

// Get returns the VFS for the current request context.
// For fixed-VFS providers it returns the single VFS.
// For proxy providers it reads the VFS from the request's auth context.
func (p *Provider) Get(ctx context.Context) (*vfs.VFS, error) {
	if p.vfs != nil {
		return p.vfs, nil
	}
	value := libhttp.CtxGetAuth(ctx)
	if value == nil {
		return nil, errors.New("no VFS found in context")
	}
	VFS, ok := value.(*vfs.VFS)
	if !ok {
		return nil, fmt.Errorf("context value is not VFS: %#v", value)
	}
	return VFS, nil
}

// VFS returns the fixed VFS, or nil if using an auth proxy.
func (p *Provider) VFS() *vfs.VFS {
	if p == nil {
		return nil
	}
	return p.vfs
}

// Proxy returns the Proxy instance, or nil if not using an auth proxy.
func (p *Provider) Proxy() *Proxy {
	if p == nil {
		return nil
	}
	return p.proxy
}

// IsProxy returns true if using an auth proxy.
func (p *Provider) IsProxy() bool {
	return p != nil && p.proxy != nil
}

// Shutdown tears down the provider: shuts down the fixed VFS or flushes all proxy cache entries.
func (p *Provider) Shutdown() {
	if p == nil {
		return
	}
	if p.vfs != nil {
		p.vfs.Shutdown()
	}
	if p.proxy != nil {
		p.proxy.Shutdown()
	}
}

// Pin pins the cache entry for key so it won't be evicted by expire
func (p *Proxy) Pin(key string) {
	if p != nil && p.vfsCache != nil {
		p.vfsCache.Pin(key)
	}
}

// Unpin unpins the cache entry for key
func (p *Proxy) Unpin(key string) {
	if p != nil && p.vfsCache != nil {
		p.vfsCache.Unpin(key)
	}
}

// Pin pins the cache entry for key if using an auth proxy
func (p *Provider) Pin(key string) {
	if p != nil && p.proxy != nil {
		p.proxy.Pin(key)
	}
}

// Unpin unpins the cache entry for key if using an auth proxy
func (p *Provider) Unpin(key string) {
	if p != nil && p.proxy != nil {
		p.proxy.Unpin(key)
	}
}
