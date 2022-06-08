// Package webdav provides an interface to the Webdav
// object storage system.
package webdav

// SetModTime might be possible
// https://stackoverflow.com/questions/3579608/webdav-can-a-client-modify-the-mtime-of-a-file
// ...support for a PROPSET to lastmodified (mind the missing get) which does the utime() call might be an option.
// For example the ownCloud WebDAV server does it that way.

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/backend/webdav/api"
	"github.com/rclone/rclone/backend/webdav/odrvcookie"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"

	ntlmssp "github.com/Azure/go-ntlmssp"
)

const (
	minSleep      = 10 * time.Millisecond
	maxSleep      = 2 * time.Second
	decayConstant = 2   // bigger for slower decay, exponential
	defaultDepth  = "1" // depth for PROPFIND
)

const defaultEncodingSharepointNTLM = (encoder.EncodeWin |
	encoder.EncodeHashPercent | // required by IIS/8.5 in contrast with onedrive which doesn't need it
	(encoder.Display &^ encoder.EncodeDot) | // test with IIS/8.5 shows that EncodeDot is not needed
	encoder.EncodeBackSlash |
	encoder.EncodeLeftSpace |
	encoder.EncodeLeftTilde |
	encoder.EncodeRightPeriod |
	encoder.EncodeRightSpace |
	encoder.EncodeInvalidUtf8)

// Register with Fs
func init() {
	configEncodingHelp := fmt.Sprintf(
		"%s\n\nDefault encoding is %s for sharepoint-ntlm or identity otherwise.",
		config.ConfigEncodingHelp, defaultEncodingSharepointNTLM)

	fs.Register(&fs.RegInfo{
		Name:        "webdav",
		Description: "Webdav",
		NewFs:       NewFs,
		Options: []fs.Option{{
			Name:     "url",
			Help:     "URL of http host to connect to.\n\nE.g. https://example.com.",
			Required: true,
		}, {
			Name: "vendor",
			Help: "Name of the Webdav site/service/software you are using.",
			Examples: []fs.OptionExample{{
				Value: "nextcloud",
				Help:  "Nextcloud",
			}, {
				Value: "owncloud",
				Help:  "Owncloud",
			}, {
				Value: "sharepoint",
				Help:  "Sharepoint Online, authenticated by Microsoft account",
			}, {
				Value: "sharepoint-ntlm",
				Help:  "Sharepoint with NTLM authentication, usually self-hosted or on-premises",
			}, {
				Value: "other",
				Help:  "Other site/service or software",
			}},
		}, {
			Name: "user",
			Help: "User name.\n\nIn case NTLM authentication is used, the username should be in the format 'Domain\\User'.",
		}, {
			Name:       "pass",
			Help:       "Password.",
			IsPassword: true,
		}, {
			Name: "bearer_token",
			Help: "Bearer token instead of user/pass (e.g. a Macaroon).",
		}, {
			Name:     "bearer_token_command",
			Help:     "Command to run to get a bearer token.",
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     configEncodingHelp,
			Advanced: true,
		}, {
			Name: "headers",
			Help: `Set HTTP headers for all transactions.

Use this to set additional HTTP headers for all transactions

The input format is comma separated list of key,value pairs.  Standard
[CSV encoding](https://godoc.org/encoding/csv) may be used.

For example, to set a Cookie use 'Cookie,name=value', or '"Cookie","name=value"'.

You can set multiple headers, e.g. '"Cookie","name=value","Authorization","xxx"'.
`,
			Default:  fs.CommaSepList{},
			Advanced: true,
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	URL                string               `config:"url"`
	Vendor             string               `config:"vendor"`
	User               string               `config:"user"`
	Pass               string               `config:"pass"`
	BearerToken        string               `config:"bearer_token"`
	BearerTokenCommand string               `config:"bearer_token_command"`
	Enc                encoder.MultiEncoder `config:"encoding"`
	Headers            fs.CommaSepList      `config:"headers"`
}

// Fs represents a remote webdav
type Fs struct {
	name               string        // name of this remote
	root               string        // the path we are working on
	opt                Options       // parsed options
	features           *fs.Features  // optional features
	endpoint           *url.URL      // URL of the host
	endpointURL        string        // endpoint as a string
	srv                *rest.Client  // the connection to the one drive server
	pacer              *fs.Pacer     // pacer for API calls
	precision          time.Duration // mod time precision
	canStream          bool          // set if can stream
	useOCMtime         bool          // set if can use X-OC-Mtime
	retryWithZeroDepth bool          // some vendors (sharepoint) won't list files when Depth is 1 (our default)
	checkBeforePurge   bool          // enables extra check that directory to purge really exists
	hasMD5             bool          // set if can use owncloud style checksums for MD5
	hasSHA1            bool          // set if can use owncloud style checksums for SHA1
	ntlmAuthMu         sync.Mutex    // mutex to serialize NTLM auth roundtrips
}

// Object describes a webdav object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs       // what this object is part of
	remote      string    // The remote path
	hasMetaData bool      // whether info below has been set
	size        int64     // size of the object
	modTime     time.Time // modification time of the object
	sha1        string    // SHA-1 of the object content if known
	md5         string    // MD5 of the object content if known
}

// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("webdav root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	423, // Locked
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func (f *Fs) shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	// If we have a bearer token command and it has expired then refresh it
	if f.opt.BearerTokenCommand != "" && resp != nil && resp.StatusCode == 401 {
		fs.Debugf(f, "Bearer token expired: %v", err)
		authErr := f.fetchAndSetBearerToken()
		if authErr != nil {
			err = authErr
		}
		return true, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// safeRoundTripper is a wrapper for http.RoundTripper that serializes
// http roundtrips. NTLM authentication sequence can involve up to four
// rounds of negotiations and might fail due to concurrency.
// This wrapper allows to use ntlmssp.Negotiator safely with goroutines.
type safeRoundTripper struct {
	fs *Fs
	rt http.RoundTripper
}

// RoundTrip guards wrapped RoundTripper by a mutex.
func (srt *safeRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	srt.fs.ntlmAuthMu.Lock()
	defer srt.fs.ntlmAuthMu.Unlock()
	return srt.rt.RoundTrip(req)
}

// itemIsDir returns true if the item is a directory
//
// When a client sees a resourcetype it doesn't recognize it should
// assume it is a regular non-collection resource.  [WebDav book by
// Lisa Dusseault ch 7.5.8 p170]
func itemIsDir(item *api.Response) bool {
	if t := item.Props.Type; t != nil {
		if t.Space == "DAV:" && t.Local == "collection" {
			return true
		}
		fs.Debugf(nil, "Unknown resource type %q/%q on %q", t.Space, t.Local, item.Props.Name)
	}
	// the iscollection prop is a Microsoft extension, but if present it is a reliable indicator
	// if the above check failed - see #2716. This can be an integer or a boolean - see #2964
	if t := item.Props.IsCollection; t != nil {
		switch x := strings.ToLower(*t); x {
		case "0", "false":
			return false
		case "1", "true":
			return true
		default:
			fs.Debugf(nil, "Unknown value %q for IsCollection", x)
		}
	}
	return false
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string, depth string) (info *api.Prop, err error) {
	// FIXME how do we read back additional properties?
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   f.filePath(path),
		ExtraHeaders: map[string]string{
			"Depth": depth,
		},
		NoRedirect: true,
	}
	if f.hasMD5 || f.hasSHA1 {
		opts.Body = bytes.NewBuffer(owncloudProps)
	}
	var result api.Multistatus
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if apiErr, ok := err.(*api.Error); ok {
		// does not exist
		switch apiErr.StatusCode {
		case http.StatusNotFound:
			if f.retryWithZeroDepth && depth != "0" {
				return f.readMetaDataForPath(ctx, path, "0")
			}
			return nil, fs.ErrorObjectNotFound
		case http.StatusMovedPermanently, http.StatusFound, http.StatusSeeOther:
			// Some sort of redirect - go doesn't deal with these properly (it resets
			// the method to GET).  However we can assume that if it was redirected the
			// object was not found.
			return nil, fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		return nil, fmt.Errorf("read metadata failed: %w", err)
	}
	if len(result.Responses) < 1 {
		return nil, fs.ErrorObjectNotFound
	}
	item := result.Responses[0]
	if !item.Props.StatusOK() {
		return nil, fs.ErrorObjectNotFound
	}
	if itemIsDir(&item) {
		return nil, fs.ErrorIsDir
	}
	return &item.Props, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	body, err := rest.ReadBody(resp)
	if err != nil {
		return fmt.Errorf("error when trying to read error from body: %w", err)
	}
	// Decode error response
	errResponse := new(api.Error)
	err = xml.Unmarshal(body, &errResponse)
	if err != nil {
		// set the Message to be the body if can't parse the XML
		errResponse.Message = strings.TrimSpace(string(body))
	}
	errResponse.Status = resp.Status
	errResponse.StatusCode = resp.StatusCode
	return errResponse
}

// addSlash makes sure s is terminated with a / if non empty
func addSlash(s string) string {
	if s != "" && !strings.HasSuffix(s, "/") {
		s += "/"
	}
	return s
}

// filePath returns a file path (f.root, file)
func (f *Fs) filePath(file string) string {
	subPath := path.Join(f.root, file)
	if f.opt.Enc != encoder.EncodeZero {
		subPath = f.opt.Enc.FromStandardPath(subPath)
	}
	return rest.URLPathEscape(subPath)
}

// dirPath returns a directory path (f.root, dir)
func (f *Fs) dirPath(dir string) string {
	return addSlash(f.filePath(dir))
}

// filePath returns a file path (f.root, remote)
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	if len(opt.Headers)%2 != 0 {
		return nil, errors.New("odd number of headers supplied")
	}
	fs.Debugf(nil, "found headers: %v", opt.Headers)

	rootIsDir := strings.HasSuffix(root, "/")
	root = strings.Trim(root, "/")

	if !strings.HasSuffix(opt.URL, "/") {
		opt.URL += "/"
	}
	if opt.Pass != "" {
		var err error
		opt.Pass, err = obscure.Reveal(opt.Pass)
		if err != nil {
			return nil, fmt.Errorf("couldn't decrypt password: %w", err)
		}
	}
	if opt.Vendor == "" {
		opt.Vendor = "other"
	}
	root = strings.Trim(root, "/")

	if opt.Enc == encoder.EncodeZero && opt.Vendor == "sharepoint-ntlm" {
		opt.Enc = defaultEncodingSharepointNTLM
	}

	// Parse the endpoint
	u, err := url.Parse(opt.URL)
	if err != nil {
		return nil, err
	}

	f := &Fs{
		name:        name,
		root:        root,
		opt:         *opt,
		endpoint:    u,
		endpointURL: u.String(),
		pacer:       fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
		precision:   fs.ModTimeNotSupported,
	}

	client := fshttp.NewClient(ctx)
	if opt.Vendor == "sharepoint-ntlm" {
		// Disable transparent HTTP/2 support as per https://golang.org/pkg/net/http/ ,
		// otherwise any connection to IIS 10.0 fails with 'stream error: stream ID 39; HTTP_1_1_REQUIRED'
		// https://docs.microsoft.com/en-us/iis/get-started/whats-new-in-iis-10/http2-on-iis says:
		// 'Windows authentication (NTLM/Kerberos/Negotiate) is not supported with HTTP/2.'
		t := fshttp.NewTransportCustom(ctx, func(t *http.Transport) {
			t.TLSNextProto = map[string]func(string, *tls.Conn) http.RoundTripper{}
		})

		// Add NTLM layer
		client.Transport = &safeRoundTripper{
			fs: f,
			rt: ntlmssp.Negotiator{RoundTripper: t},
		}
	}
	f.srv = rest.NewClient(client).SetRoot(u.String())

	f.features = (&fs.Features{
		CanHaveEmptyDirectories: true,
	}).Fill(ctx, f)
	if opt.User != "" || opt.Pass != "" {
		f.srv.SetUserPass(opt.User, opt.Pass)
	} else if opt.BearerToken != "" {
		f.setBearerToken(opt.BearerToken)
	} else if f.opt.BearerTokenCommand != "" {
		err = f.fetchAndSetBearerToken()
		if err != nil {
			return nil, err
		}
	}
	if opt.Headers != nil {
		f.addHeaders(opt.Headers)
	}
	f.srv.SetErrorHandler(errorHandler)
	err = f.setQuirks(ctx, opt.Vendor)
	if err != nil {
		return nil, err
	}
	if !f.findHeader(opt.Headers, "Referer") {
		f.srv.SetHeader("Referer", u.String())
	}

	if root != "" && !rootIsDir {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(ctx, remote)
		if err != nil {
			if errors.Is(err, fs.ErrorObjectNotFound) || errors.Is(err, fs.ErrorIsDir) {
				// File doesn't exist so return old f
				f.root = root
				return f, nil
			}
			return nil, err
		}
		// return an error with an fs which points to the parent
		return f, fs.ErrorIsFile
	}
	return f, nil
}

// sets the BearerToken up
func (f *Fs) setBearerToken(token string) {
	f.opt.BearerToken = token
	f.srv.SetHeader("Authorization", "Bearer "+token)
}

// fetch the bearer token using the command
func (f *Fs) fetchBearerToken(cmd string) (string, error) {
	var (
		args   = strings.Split(cmd, " ")
		stdout bytes.Buffer
		stderr bytes.Buffer
		c      = exec.Command(args[0], args[1:]...)
	)
	c.Stdout = &stdout
	c.Stderr = &stderr
	var (
		err          = c.Run()
		stdoutString = strings.TrimSpace(stdout.String())
		stderrString = strings.TrimSpace(stderr.String())
	)
	if err != nil {
		if stderrString == "" {
			stderrString = stdoutString
		}
		return "", fmt.Errorf("failed to get bearer token using %q: %s: %w", f.opt.BearerTokenCommand, stderrString, err)
	}
	return stdoutString, nil
}

// Adds the configured headers to the request if any
func (f *Fs) addHeaders(headers fs.CommaSepList) {
	for i := 0; i < len(headers); i += 2 {
		key := f.opt.Headers[i]
		value := f.opt.Headers[i+1]
		f.srv.SetHeader(key, value)
	}
}

// Returns true if the header was configured
func (f *Fs) findHeader(headers fs.CommaSepList, find string) bool {
	for i := 0; i < len(headers); i += 2 {
		key := f.opt.Headers[i]
		if strings.EqualFold(key, find) {
			return true
		}
	}
	return false
}

// fetch the bearer token and set it if successful
func (f *Fs) fetchAndSetBearerToken() error {
	if f.opt.BearerTokenCommand == "" {
		return nil
	}
	token, err := f.fetchBearerToken(f.opt.BearerTokenCommand)
	if err != nil {
		return err
	}
	f.setBearerToken(token)
	return nil
}

// setQuirks adjusts the Fs for the vendor passed in
func (f *Fs) setQuirks(ctx context.Context, vendor string) error {
	switch vendor {
	case "owncloud":
		f.canStream = true
		f.precision = time.Second
		f.useOCMtime = true
		f.hasMD5 = true
		f.hasSHA1 = true
	case "nextcloud":
		f.precision = time.Second
		f.useOCMtime = true
		f.hasSHA1 = true
	case "sharepoint":
		// To mount sharepoint, two Cookies are required
		// They have to be set instead of BasicAuth
		f.srv.RemoveHeader("Authorization") // We don't need this Header if using cookies
		spCk := odrvcookie.New(f.opt.User, f.opt.Pass, f.endpointURL)
		spCookies, err := spCk.Cookies(ctx)
		if err != nil {
			return err
		}

		odrvcookie.NewRenew(12*time.Hour, func() {
			spCookies, err := spCk.Cookies(ctx)
			if err != nil {
				fs.Errorf("could not renew cookies: %s", err.Error())
				return
			}
			f.srv.SetCookie(&spCookies.FedAuth, &spCookies.RtFa)
			fs.Debugf(spCookies, "successfully renewed sharepoint cookies")
		})

		f.srv.SetCookie(&spCookies.FedAuth, &spCookies.RtFa)

		// sharepoint, unlike the other vendors, only lists files if the depth header is set to 0
		// however, rclone defaults to 1 since it provides recursive directory listing
		// to determine if we may have found a file, the request has to be resent
		// with the depth set to 0
		f.retryWithZeroDepth = true
	case "sharepoint-ntlm":
		// Sharepoint with NTLM authentication
		// See comment above
		f.retryWithZeroDepth = true

		// Sharepoint 2016 returns status 204 to the purge request
		// even if the directory to purge does not really exist
		// so we must perform an extra check to detect this
		// condition and return a proper error code.
		f.checkBeforePurge = true
	case "other":
	default:
		fs.Debugf(f, "Unknown vendor %q", vendor)
	}

	// Remove PutStream from optional features
	if !f.canStream {
		f.features.PutStream = nil
	}
	return nil
}

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.Prop) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx) // reads info and meta, returning an error
	}
	if err != nil {
		return nil, err
	}
	return o, nil
}

// NewObject finds the Object at remote.  If it can't be found
// it returns the error fs.ErrorObjectNotFound.
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.newObjectWithInfo(ctx, remote, nil)
}

// Read the normal props, plus the checksums
//
// <oc:checksums><oc:checksum>SHA1:f572d396fae9206628714fb2ce00f72e94f2258f MD5:b1946ac92492d2347c6235b4d2611184 ADLER32:084b021f</oc:checksum></oc:checksums>
var owncloudProps = []byte(`<?xml version="1.0"?>
<d:propfind  xmlns:d="DAV:" xmlns:oc="http://owncloud.org/ns" xmlns:nc="http://nextcloud.org/ns">
 <d:prop>
  <d:displayname />
  <d:getlastmodified />
  <d:getcontentlength />
  <d:resourcetype />
  <d:getcontenttype />
  <oc:checksums />
 </d:prop>
</d:propfind>
`)

// list the objects into the function supplied
//
// If directories is set it only sends directories
// User function to process a File item from listAll
//
// Should return true to finish processing
type listAllFn func(string, bool, *api.Prop) bool

// Lists the directory required calling the user function on each item found
//
// If the user fn ever returns true then it early exits with found = true
func (f *Fs) listAll(ctx context.Context, dir string, directoriesOnly bool, filesOnly bool, depth string, fn listAllFn) (found bool, err error) {
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   f.dirPath(dir), // FIXME Should not start with /
		ExtraHeaders: map[string]string{
			"Depth": depth,
		},
	}
	if f.hasMD5 || f.hasSHA1 {
		opts.Body = bytes.NewBuffer(owncloudProps)
	}
	var result api.Multistatus
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				if f.retryWithZeroDepth && depth != "0" {
					return f.listAll(ctx, dir, directoriesOnly, filesOnly, "0", fn)
				}
				return found, fs.ErrorDirNotFound
			}
		}
		return found, fmt.Errorf("couldn't list files: %w", err)
	}
	//fmt.Printf("result = %#v", &result)
	baseURL, err := rest.URLJoin(f.endpoint, opts.Path)
	if err != nil {
		return false, fmt.Errorf("couldn't join URL: %w", err)
	}
	for i := range result.Responses {
		item := &result.Responses[i]
		isDir := itemIsDir(item)

		// Find name
		u, err := rest.URLJoin(baseURL, item.Href)
		if err != nil {
			fs.Errorf(nil, "URL Join failed for %q and %q: %v", baseURL, item.Href, err)
			continue
		}
		// Make sure directories end with a /
		if isDir {
			u.Path = addSlash(u.Path)
		}
		if !strings.HasPrefix(u.Path, baseURL.Path) {
			fs.Debugf(nil, "Item with unknown path received: %q, %q", u.Path, baseURL.Path)
			continue
		}
		subPath := u.Path[len(baseURL.Path):]
		if f.opt.Enc != encoder.EncodeZero {
			subPath = f.opt.Enc.ToStandardPath(subPath)
		}
		remote := path.Join(dir, subPath)
		remote = strings.TrimSuffix(remote, "/")

		// the listing contains info about itself which we ignore
		if remote == dir {
			continue
		}

		// Check OK
		if !item.Props.StatusOK() {
			fs.Debugf(remote, "Ignoring item with bad status %q", item.Props.Status)
			continue
		}

		if isDir {
			if filesOnly {
				continue
			}
		} else {
			if directoriesOnly {
				continue
			}
		}
		// 	item.Name = restoreReservedChars(item.Name)
		if fn(remote, isDir, &item.Props) {
			found = true
			break
		}
	}
	return
}

// List the objects and directories in dir into entries.  The
// entries can be returned in any order but should be for a
// complete directory.
//
// dir should be "" to list the root, and should not have
// trailing slashes.
//
// This should return ErrDirNotFound if the directory isn't
// found.
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	var iErr error
	_, err = f.listAll(ctx, dir, false, false, defaultDepth, func(remote string, isDir bool, info *api.Prop) bool {
		if isDir {
			d := fs.NewDir(remote, time.Time(info.Modified))
			// .SetID(info.ID)
			// FIXME more info from dir? can set size, items?
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, info)
			if err != nil {
				iErr = err
				return true
			}
			entries = append(entries, o)
		}
		return false
	})
	if err != nil {
		return nil, err
	}
	if iErr != nil {
		return nil, iErr
	}
	return entries, nil
}

// Creates from the parameters passed in a half finished Object which
// must have setMetaData called on it
//
// Used to create new objects
func (f *Fs) createObject(remote string, modTime time.Time, size int64) (o *Object) {
	// Temporary Object under construction
	o = &Object{
		fs:      f,
		remote:  remote,
		size:    size,
		modTime: modTime,
	}
	return o
}

// Put the object
//
// Copy the reader in to the new object which is returned
//
// The new object may have been created if an error is returned
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	o := f.createObject(src.Remote(), src.ModTime(ctx), src.Size())
	return o, o.Update(ctx, in, src, options...)
}

// PutStream uploads to the remote path with the modTime given of indeterminate size
func (f *Fs) PutStream(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	return f.Put(ctx, in, src, options...)
}

// mkParentDir makes the parent of the native path dirPath if
// necessary and any directories above that
func (f *Fs) mkParentDir(ctx context.Context, dirPath string) (err error) {
	// defer log.Trace(dirPath, "")("err=%v", &err)
	// chop off trailing / if it exists
	parent := path.Dir(strings.TrimSuffix(dirPath, "/"))
	if parent == "." {
		parent = ""
	}
	return f.mkdir(ctx, parent)
}

// _dirExists - list dirPath to see if it exists
//
// dirPath should be a native path ending in a /
func (f *Fs) _dirExists(ctx context.Context, dirPath string) (exists bool) {
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   dirPath,
		ExtraHeaders: map[string]string{
			"Depth": "0",
		},
	}
	var result api.Multistatus
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return f.shouldRetry(ctx, resp, err)
	})
	return err == nil
}

// low level mkdir, only makes the directory, doesn't attempt to create parents
func (f *Fs) _mkdir(ctx context.Context, dirPath string) error {
	// We assume the root is already created
	if dirPath == "" {
		return nil
	}
	// Collections must end with /
	if !strings.HasSuffix(dirPath, "/") {
		dirPath += "/"
	}
	opts := rest.Opts{
		Method:     "MKCOL",
		Path:       dirPath,
		NoResponse: true,
	}
	err := f.pacer.Call(func() (bool, error) {
		resp, err := f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if apiErr, ok := err.(*api.Error); ok {
		// Check if it already exists. The response code for this isn't
		// defined in the RFC so the implementations vary wildly.
		//
		// owncloud returns 423/StatusLocked if the create is already in progress
		if apiErr.StatusCode == http.StatusMethodNotAllowed || apiErr.StatusCode == http.StatusNotAcceptable || apiErr.StatusCode == http.StatusLocked {
			return nil
		}
		// 4shared returns a 409/StatusConflict here which clashes
		// horribly with the intermediate paths don't exist meaning. So
		// check to see if actually exists. This will correct other
		// error codes too.
		if f._dirExists(ctx, dirPath) {
			return nil
		}

	}
	return err
}

// mkdir makes the directory and parents using native paths
func (f *Fs) mkdir(ctx context.Context, dirPath string) (err error) {
	// defer log.Trace(dirPath, "")("err=%v", &err)
	err = f._mkdir(ctx, dirPath)
	if apiErr, ok := err.(*api.Error); ok {
		// parent does not exist so create it first then try again
		if apiErr.StatusCode == http.StatusConflict {
			err = f.mkParentDir(ctx, dirPath)
			if err == nil {
				err = f._mkdir(ctx, dirPath)
			}
		}
	}
	return err
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	dirPath := f.dirPath(dir)
	return f.mkdir(ctx, dirPath)
}

// dirNotEmpty returns true if the directory exists and is not Empty
//
// if the directory does not exist then err will be ErrorDirNotFound
func (f *Fs) dirNotEmpty(ctx context.Context, dir string) (found bool, err error) {
	return f.listAll(ctx, dir, false, false, defaultDepth, func(remote string, isDir bool, info *api.Prop) bool {
		return true
	})
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) error {
	if check {
		notEmpty, err := f.dirNotEmpty(ctx, dir)
		if err != nil {
			return err
		}
		if notEmpty {
			return fs.ErrorDirectoryNotEmpty
		}
	} else if f.checkBeforePurge {
		// We are doing purge as the `check` argument is unset.
		// The quirk says that we are working with Sharepoint 2016.
		// This provider returns status 204 even if the purged directory
		// does not really exist so we perform an extra check here.
		// Only the existence is checked, all other errors must be
		// ignored here to make the rclone test suite pass.
		depth := defaultDepth
		if f.retryWithZeroDepth {
			depth = "0"
		}
		_, err := f.readMetaDataForPath(ctx, dir, depth)
		if err == fs.ErrorObjectNotFound {
			return fs.ErrorDirNotFound
		}
	}
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       f.dirPath(dir),
		NoResponse: true,
	}
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, nil)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("rmdir failed: %w", err)
	}
	// FIXME parse Multistatus response
	return nil
}

// Rmdir deletes the root folder
//
// Returns an error if it isn't empty
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, true)
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return f.precision
}

// Copy or Move src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy/fs.ErrorCantMove
func (f *Fs) copyOrMove(ctx context.Context, src fs.Object, remote string, method string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		if method == "COPY" {
			return nil, fs.ErrorCantCopy
		}
		return nil, fs.ErrorCantMove
	}
	dstPath := f.filePath(remote)
	err := f.mkParentDir(ctx, dstPath)
	if err != nil {
		return nil, fmt.Errorf("Copy mkParentDir failed: %w", err)
	}
	destinationURL, err := rest.URLJoin(f.endpoint, dstPath)
	if err != nil {
		return nil, fmt.Errorf("copyOrMove couldn't join URL: %w", err)
	}
	var resp *http.Response
	opts := rest.Opts{
		Method:     method,
		Path:       srcObj.filePath(),
		NoResponse: true,
		ExtraHeaders: map[string]string{
			"Destination": destinationURL.String(),
			"Overwrite":   "F",
		},
	}
	if f.useOCMtime {
		opts.ExtraHeaders["X-OC-Mtime"] = fmt.Sprintf("%d", src.ModTime(ctx).Unix())
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, fmt.Errorf("Copy call failed: %w", err)
	}
	dstObj, err := f.NewObject(ctx, remote)
	if err != nil {
		return nil, fmt.Errorf("Copy NewObject failed: %w", err)
	}
	return dstObj, nil
}

// Copy src to this remote using server-side copy operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantCopy
func (f *Fs) Copy(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(ctx, src, remote, "COPY")
}

// Purge deletes all the files in the directory
//
// Optional interface: Only implement this if you have a way of
// deleting all the files quicker than just running Remove() on the
// result of List()
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// Move src to this remote using server-side move operations.
//
// This is stored with the remote path given
//
// It returns the destination Object and a possible error
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantMove
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	return f.copyOrMove(ctx, src, remote, "MOVE")
}

// DirMove moves src, srcRemote to this remote at dstRemote
// using server-side move operations.
//
// Will only be called if src.Fs().Name() == f.Name()
//
// If it isn't possible then return fs.ErrorCantDirMove
//
// If destination exists then return fs.ErrorDirExists
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}
	srcPath := srcFs.filePath(srcRemote)
	dstPath := f.filePath(dstRemote)

	// Check if destination exists
	_, err := f.dirNotEmpty(ctx, dstRemote)
	if err == nil {
		return fs.ErrorDirExists
	}
	if err != fs.ErrorDirNotFound {
		return fmt.Errorf("DirMove dirExists dst failed: %w", err)
	}

	// Make sure the parent directory exists
	err = f.mkParentDir(ctx, dstPath)
	if err != nil {
		return fmt.Errorf("DirMove mkParentDir dst failed: %w", err)
	}

	destinationURL, err := rest.URLJoin(f.endpoint, dstPath)
	if err != nil {
		return fmt.Errorf("DirMove couldn't join URL: %w", err)
	}

	var resp *http.Response
	opts := rest.Opts{
		Method:     "MOVE",
		Path:       addSlash(srcPath),
		NoResponse: true,
		ExtraHeaders: map[string]string{
			"Destination": addSlash(destinationURL.String()),
			"Overwrite":   "F",
		},
	}
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return fmt.Errorf("DirMove MOVE call failed: %w", err)
	}
	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	hashes := hash.Set(hash.None)
	if f.hasMD5 {
		hashes.Add(hash.MD5)
	}
	if f.hasSHA1 {
		hashes.Add(hash.SHA1)
	}
	return hashes
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	opts := rest.Opts{
		Method: "PROPFIND",
		Path:   "",
		ExtraHeaders: map[string]string{
			"Depth": "0",
		},
	}
	opts.Body = bytes.NewBuffer([]byte(`<?xml version="1.0" ?>
<D:propfind xmlns:D="DAV:">
 <D:prop>
  <D:quota-available-bytes/>
  <D:quota-used-bytes/>
 </D:prop>
</D:propfind>
`))
	var q api.Quota
	var resp *http.Response
	var err error
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &q)
		return f.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	usage := &fs.Usage{}
	if i, err := strconv.ParseInt(q.Used, 10, 64); err == nil && i >= 0 {
		usage.Used = fs.NewUsageValue(i)
	}
	if i, err := strconv.ParseInt(q.Available, 10, 64); err == nil && i >= 0 {
		usage.Free = fs.NewUsageValue(i)
	}
	if usage.Used != nil && usage.Free != nil {
		usage.Total = fs.NewUsageValue(*usage.Used + *usage.Free)
	}
	return usage, nil
}

// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Return a string version
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// Hash returns the SHA1 or MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t == hash.MD5 && o.fs.hasMD5 {
		return o.md5, nil
	}
	if t == hash.SHA1 && o.fs.hasSHA1 {
		return o.sha1, nil
	}
	return "", hash.ErrUnsupported
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	ctx := context.TODO()
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.Prop) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.modTime = time.Time(info.Modified)
	if o.fs.hasMD5 || o.fs.hasSHA1 {
		hashes := info.Hashes()
		if o.fs.hasSHA1 {
			o.sha1 = hashes[hash.SHA1]
		}
		if o.fs.hasMD5 {
			o.md5 = hashes[hash.MD5]
		}
	}
	return nil
}

// readMetaData gets the metadata if it hasn't already been fetched
//
// it also sets the info
func (o *Object) readMetaData(ctx context.Context) (err error) {
	if o.hasMetaData {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote, defaultDepth)
	if err != nil {
		return err
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return time.Now()
	}
	return o.modTime
}

// SetModTime sets the modification time of the local fs object
func (o *Object) SetModTime(ctx context.Context, modTime time.Time) error {
	return fs.ErrorCantSetModTime
}

// Storable returns a boolean showing whether this object storable
func (o *Object) Storable() bool {
	return true
}

// Open an object for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (in io.ReadCloser, err error) {
	var resp *http.Response
	fs.FixRangeOption(options, o.size)
	opts := rest.Opts{
		Method:  "GET",
		Path:    o.filePath(),
		Options: options,
		ExtraHeaders: map[string]string{
			"Depth": "0",
		},
	}
	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	err = o.fs.mkParentDir(ctx, o.filePath())
	if err != nil {
		return fmt.Errorf("Update mkParentDir failed: %w", err)
	}

	size := src.Size()
	var resp *http.Response
	opts := rest.Opts{
		Method:        "PUT",
		Path:          o.filePath(),
		Body:          in,
		NoResponse:    true,
		ContentLength: &size, // FIXME this isn't necessary with owncloud - See https://github.com/nextcloud/nextcloud-snap/issues/365
		ContentType:   fs.MimeType(ctx, src),
		Options:       options,
	}
	if o.fs.useOCMtime || o.fs.hasMD5 || o.fs.hasSHA1 {
		opts.ExtraHeaders = map[string]string{}
		if o.fs.useOCMtime {
			opts.ExtraHeaders["X-OC-Mtime"] = fmt.Sprintf("%d", src.ModTime(ctx).Unix())
		}
		// Set one upload checksum
		// Owncloud uses one checksum only to check the upload and stores its own SHA1 and MD5
		// Nextcloud stores the checksum you supply (SHA1 or MD5) but only stores one
		if o.fs.hasSHA1 {
			if sha1, _ := src.Hash(ctx, hash.SHA1); sha1 != "" {
				opts.ExtraHeaders["OC-Checksum"] = "SHA1:" + sha1
			}
		}
		if o.fs.hasMD5 && opts.ExtraHeaders["OC-Checksum"] == "" {
			if md5, _ := src.Hash(ctx, hash.MD5); md5 != "" {
				opts.ExtraHeaders["OC-Checksum"] = "MD5:" + md5
			}
		}
	}
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
	if err != nil {
		// Give the WebDAV server a chance to get its internal state in order after the
		// error.  The error may have been local in which case we closed the connection.
		// The server may still be dealing with it for a moment. A sleep isn't ideal but I
		// haven't been able to think of a better method to find out if the server has
		// finished - ncw
		time.Sleep(1 * time.Second)
		// Remove failed upload
		_ = o.Remove(ctx)
		return err
	}
	// read metadata from remote
	o.hasMetaData = false
	return o.readMetaData(ctx)
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method:     "DELETE",
		Path:       o.filePath(),
		NoResponse: true,
	}
	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.Call(ctx, &opts)
		return o.fs.shouldRetry(ctx, resp, err)
	})
}

// Check the interfaces are satisfied
var (
	_ fs.Fs          = (*Fs)(nil)
	_ fs.Purger      = (*Fs)(nil)
	_ fs.PutStreamer = (*Fs)(nil)
	_ fs.Copier      = (*Fs)(nil)
	_ fs.Mover       = (*Fs)(nil)
	_ fs.DirMover    = (*Fs)(nil)
	_ fs.Abouter     = (*Fs)(nil)
	_ fs.Object      = (*Object)(nil)
)
