package jottacloud

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/backend/jottacloud/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/accounting"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/walk"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/oauthutil"
	"github.com/rclone/rclone/lib/pacer"
	"github.com/rclone/rclone/lib/rest"
	"golang.org/x/oauth2"
)

// Globals
const (
	minSleep           = 10 * time.Millisecond
	maxSleep           = 2 * time.Second
	decayConstant      = 2 // bigger for slower decay, exponential
	defaultDevice      = "Jotta"
	defaultMountpoint  = "Archive"
	rootURL            = "https://jfs.jottacloud.com/jfs/"
	apiURL             = "https://api.jottacloud.com/"
	baseURL            = "https://www.jottacloud.com/"
	cachePrefix        = "rclone-jcmd5-"
	configDevice       = "device"
	configMountpoint   = "mountpoint"
	configTokenURL     = "tokenURL"
	configClientID     = "client_id"
	configClientSecret = "client_secret"
	configUsername     = "username"
	configVersion      = 1

	defaultTokenURL = "https://id.jottacloud.com/auth/realms/jottacloud/protocol/openid-connect/token"
	defaultClientID = "jottacli"

	legacyTokenURL              = "https://api.jottacloud.com/auth/v1/token"
	legacyRegisterURL           = "https://api.jottacloud.com/auth/v1/register"
	legacyClientID              = "nibfk8biu12ju7hpqomr8b1e40"
	legacyEncryptedClientSecret = "Vp8eAv7eVElMnQwN-kgU9cbhgApNDaMqWdlDi5qFydlQoji4JBxrGMF2"
	legacyConfigVersion         = 0

	teliaCloudTokenURL = "https://cloud-auth.telia.se/auth/realms/telia_se/protocol/openid-connect/token"
	teliaCloudAuthURL  = "https://cloud-auth.telia.se/auth/realms/telia_se/protocol/openid-connect/auth"
	teliaCloudClientID = "desktop"
)

// Register with Fs
func init() {
	// needs to be done early so we can use oauth during config
	fs.Register(&fs.RegInfo{
		Name:        "jottacloud",
		Description: "Jottacloud",
		NewFs:       NewFs,
		Config:      Config,
		Options: []fs.Option{{
			Name:     "md5_memory_limit",
			Help:     "Files bigger than this will be cached on disk to calculate the MD5 if required.",
			Default:  fs.SizeSuffix(10 * 1024 * 1024),
			Advanced: true,
		}, {
			Name:     "trashed_only",
			Help:     "Only show files that are in the trash.\nThis will show trashed files in their original directory structure.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "hard_delete",
			Help:     "Delete files permanently rather than putting them into the trash.",
			Default:  false,
			Advanced: true,
		}, {
			Name:     "upload_resume_limit",
			Help:     "Files bigger than this can be resumed if the upload fail's.",
			Default:  fs.SizeSuffix(10 * 1024 * 1024),
			Advanced: true,
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			// Encode invalid UTF-8 bytes as xml doesn't handle them properly.
			//
			// Also: '*', '/', ':', '<', '>', '?', '\"', '\x00', '|'
			Default: (encoder.Display |
				encoder.EncodeWin | // :?"*<>|
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Config runs the backend configuration protocol
func Config(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
	switch config.State {
	case "":
		return fs.ConfigChooseFixed("auth_type_done", "config_type", `Authentication type`, []fs.OptionExample{{
			Value: "standard",
			Help:  "Standard authentication - use this if you're a normal Jottacloud user.",
		}, {
			Value: "legacy",
			Help:  "Legacy authentication - this is only required for certain whitelabel versions of Jottacloud and not recommended for normal users.",
		}, {
			Value: "telia",
			Help:  "Telia Cloud authentication - use this if you are using Telia Cloud.",
		}})
	case "auth_type_done":
		// Jump to next state according to config chosen
		return fs.ConfigGoto(config.Result)
	case "standard": // configure a jottacloud backend using the modern JottaCli token based authentication
		m.Set("configVersion", fmt.Sprint(configVersion))
		return fs.ConfigInput("standard_token", "config_login_token", "Personal login token.\n\nGenerate here: https://www.jottacloud.com/web/secure")
	case "standard_token":
		loginToken := config.Result
		m.Set(configClientID, defaultClientID)
		m.Set(configClientSecret, "")

		srv := rest.NewClient(fshttp.NewClient(ctx))
		token, tokenEndpoint, err := doTokenAuth(ctx, srv, loginToken)
		if err != nil {
			return nil, errors.Wrap(err, "failed to get oauth token")
		}
		m.Set(configTokenURL, tokenEndpoint)
		err = oauthutil.PutToken(name, m, &token, true)
		if err != nil {
			return nil, errors.Wrap(err, "error while saving token")
		}
		return fs.ConfigGoto("choose_device")
	case "legacy": // configure a jottacloud backend using legacy authentication
		m.Set("configVersion", fmt.Sprint(legacyConfigVersion))
		return fs.ConfigConfirm("legacy_api", false, "config_machine_specific", `Do you want to create a machine specific API key?

Rclone has it's own Jottacloud API KEY which works fine as long as one
only uses rclone on a single machine. When you want to use rclone with
this account on more than one machine it's recommended to create a
machine specific API key. These keys can NOT be shared between
machines.`)
	case "legacy_api":
		srv := rest.NewClient(fshttp.NewClient(ctx))
		if config.Result == "true" {
			deviceRegistration, err := registerDevice(ctx, srv)
			if err != nil {
				return nil, errors.Wrap(err, "failed to register device")
			}
			m.Set(configClientID, deviceRegistration.ClientID)
			m.Set(configClientSecret, obscure.MustObscure(deviceRegistration.ClientSecret))
			fs.Debugf(nil, "Got clientID %q and clientSecret %q", deviceRegistration.ClientID, deviceRegistration.ClientSecret)
		}
		return fs.ConfigInput("legacy_username", "config_username", "Username (e-mail address)")
	case "legacy_username":
		m.Set(configUsername, config.Result)
		return fs.ConfigPassword("legacy_password", "config_password", "Password (only used in setup, will not be stored)")
	case "legacy_password":
		m.Set("password", config.Result)
		m.Set("auth_code", "")
		return fs.ConfigGoto("legacy_do_auth")
	case "legacy_auth_code":
		authCode := strings.Replace(config.Result, "-", "", -1) // remove any "-" contained in the code so we have a 6 digit number
		m.Set("auth_code", authCode)
		return fs.ConfigGoto("legacy_do_auth")
	case "legacy_do_auth":
		username, _ := m.Get(configUsername)
		password, _ := m.Get("password")
		password = obscure.MustReveal(password)
		authCode, _ := m.Get("auth_code")

		srv := rest.NewClient(fshttp.NewClient(ctx))
		clientID, ok := m.Get(configClientID)
		if !ok {
			clientID = legacyClientID
		}
		clientSecret, ok := m.Get(configClientSecret)
		if !ok {
			clientSecret = legacyEncryptedClientSecret
		}

		oauthConfig := &oauth2.Config{
			Endpoint: oauth2.Endpoint{
				AuthURL: legacyTokenURL,
			},
			ClientID:     clientID,
			ClientSecret: obscure.MustReveal(clientSecret),
		}
		token, err := doLegacyAuth(ctx, srv, oauthConfig, username, password, authCode)
		if err == errAuthCodeRequired {
			return fs.ConfigInput("legacy_auth_code", "config_auth_code", "Verification Code\nThis account uses 2 factor authentication you will receive a verification code via SMS.")
		}
		m.Set("password", "")
		m.Set("auth_code", "")
		if err != nil {
			return nil, errors.Wrap(err, "failed to get oauth token")
		}
		err = oauthutil.PutToken(name, m, &token, true)
		if err != nil {
			return nil, errors.Wrap(err, "error while saving token")
		}
		return fs.ConfigGoto("choose_device")
	case "telia": // telia cloud config
		m.Set("configVersion", fmt.Sprint(configVersion))
		m.Set(configClientID, teliaCloudClientID)
		m.Set(configTokenURL, teliaCloudTokenURL)
		return oauthutil.ConfigOut("choose_device", &oauthutil.Options{
			OAuth2Config: &oauth2.Config{
				Endpoint: oauth2.Endpoint{
					AuthURL:  teliaCloudAuthURL,
					TokenURL: teliaCloudTokenURL,
				},
				ClientID:    teliaCloudClientID,
				Scopes:      []string{"openid", "jotta-default", "offline_access"},
				RedirectURL: oauthutil.RedirectLocalhostURL,
			},
		})
	case "choose_device":
		return fs.ConfigConfirm("choose_device_query", false, "config_non_standard", "Use a non standard device/mountpoint e.g. for accessing files uploaded using the official Jottacloud client?")
	case "choose_device_query":
		if config.Result != "true" {
			m.Set(configDevice, "")
			m.Set(configMountpoint, "")
			return fs.ConfigGoto("end")
		}
		oAuthClient, _, err := getOAuthClient(ctx, name, m)
		if err != nil {
			return nil, err
		}
		srv := rest.NewClient(oAuthClient).SetRoot(rootURL)
		apiSrv := rest.NewClient(oAuthClient).SetRoot(apiURL)

		cust, err := getCustomerInfo(ctx, apiSrv)
		if err != nil {
			return nil, err
		}
		m.Set(configUsername, cust.Username)

		acc, err := getDriveInfo(ctx, srv, cust.Username)
		if err != nil {
			return nil, err
		}
		return fs.ConfigChoose("choose_device_result", "config_device", `Please select the device to use. Normally this will be Jotta`, len(acc.Devices), func(i int) (string, string) {
			return acc.Devices[i].Name, ""
		})
	case "choose_device_result":
		device := config.Result
		m.Set(configDevice, device)

		oAuthClient, _, err := getOAuthClient(ctx, name, m)
		if err != nil {
			return nil, err
		}
		srv := rest.NewClient(oAuthClient).SetRoot(rootURL)

		username, _ := m.Get(configUsername)
		dev, err := getDeviceInfo(ctx, srv, path.Join(username, device))
		if err != nil {
			return nil, err
		}
		return fs.ConfigChoose("choose_device_mountpoint", "config_mountpoint", `Please select the mountpoint to use. Normally this will be Archive.`, len(dev.MountPoints), func(i int) (string, string) {
			return dev.MountPoints[i].Name, ""
		})
	case "choose_device_mountpoint":
		mountpoint := config.Result
		m.Set(configMountpoint, mountpoint)
		return fs.ConfigGoto("end")
	case "end":
		// All the config flows end up here in case we need to carry on with something
		return nil, nil
	}
	return nil, fmt.Errorf("unknown state %q", config.State)
}

// Options defines the configuration for this backend
type Options struct {
	Device             string               `config:"device"`
	Mountpoint         string               `config:"mountpoint"`
	MD5MemoryThreshold fs.SizeSuffix        `config:"md5_memory_limit"`
	TrashedOnly        bool                 `config:"trashed_only"`
	HardDelete         bool                 `config:"hard_delete"`
	UploadThreshold    fs.SizeSuffix        `config:"upload_resume_limit"`
	Enc                encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote jottacloud
type Fs struct {
	name         string
	root         string
	user         string
	opt          Options
	features     *fs.Features
	endpointURL  string
	srv          *rest.Client
	apiSrv       *rest.Client
	pacer        *fs.Pacer
	tokenRenewer *oauthutil.Renew // renew the token on expiry
}

// Object describes a jottacloud object
//
// Will definitely have info but maybe not meta
type Object struct {
	fs          *Fs
	remote      string
	hasMetaData bool
	size        int64
	modTime     time.Time
	md5         string
	mimeType    string
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
	return fmt.Sprintf("jottacloud root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// joinPath joins two path/url elements
//
// Does not perform clean on the result like path.Join does,
// which breaks urls by changing prefix "https://" into "https:/".
func joinPath(base string, rel string) string {
	if rel == "" {
		return base
	}
	if strings.HasSuffix(base, "/") {
		return base + strings.TrimPrefix(rel, "/")
	}
	if strings.HasPrefix(rel, "/") {
		return strings.TrimSuffix(base, "/") + rel
	}
	return base + "/" + rel
}

// retryErrorCodes is a slice of error codes that we will retry
var retryErrorCodes = []int{
	429, // Too Many Requests.
	500, // Internal Server Error
	502, // Bad Gateway
	503, // Service Unavailable
	504, // Gateway Timeout
	509, // Bandwidth Limit Exceeded
}

// shouldRetry returns a boolean as to whether this resp and err
// deserve to be retried.  It returns the err as a convenience
func shouldRetry(ctx context.Context, resp *http.Response, err error) (bool, error) {
	if fserrors.ContextError(ctx, &err) {
		return false, err
	}
	return fserrors.ShouldRetry(err) || fserrors.ShouldRetryHTTP(resp, retryErrorCodes), err
}

// registerDevice register a new device for use with the jottacloud API
func registerDevice(ctx context.Context, srv *rest.Client) (reg *api.DeviceRegistrationResponse, err error) {
	// random generator to generate random device names
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	randonDeviceNamePartLength := 21
	randomDeviceNamePart := make([]byte, randonDeviceNamePartLength)
	charset := "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	for i := range randomDeviceNamePart {
		randomDeviceNamePart[i] = charset[seededRand.Intn(len(charset))]
	}
	randomDeviceName := "rclone-" + string(randomDeviceNamePart)
	fs.Debugf(nil, "Trying to register device '%s'", randomDeviceName)

	values := url.Values{}
	values.Set("device_id", randomDeviceName)

	opts := rest.Opts{
		Method:       "POST",
		RootURL:      legacyRegisterURL,
		ContentType:  "application/x-www-form-urlencoded",
		ExtraHeaders: map[string]string{"Authorization": "Bearer c2xrZmpoYWRsZmFramhkc2xma2phaHNkbGZramhhc2xkZmtqaGFzZGxrZmpobGtq"},
		Parameters:   values,
	}

	var deviceRegistration *api.DeviceRegistrationResponse
	_, err = srv.CallJSON(ctx, &opts, nil, &deviceRegistration)
	return deviceRegistration, err
}

var errAuthCodeRequired = errors.New("auth code required")

// doLegacyAuth runs the actual token request for V1 authentication
//
// Call this first with blank authCode. If errAuthCodeRequired is
// returned then call it again with an authCode
func doLegacyAuth(ctx context.Context, srv *rest.Client, oauthConfig *oauth2.Config, username, password, authCode string) (token oauth2.Token, err error) {
	// prepare out token request with username and password
	values := url.Values{}
	values.Set("grant_type", "PASSWORD")
	values.Set("password", password)
	values.Set("username", username)
	values.Set("client_id", oauthConfig.ClientID)
	values.Set("client_secret", oauthConfig.ClientSecret)
	opts := rest.Opts{
		Method:      "POST",
		RootURL:     oauthConfig.Endpoint.AuthURL,
		ContentType: "application/x-www-form-urlencoded",
		Parameters:  values,
	}
	if authCode != "" {
		opts.ExtraHeaders = make(map[string]string)
		opts.ExtraHeaders["X-Jottacloud-Otp"] = authCode
	}

	// do the first request
	var jsonToken api.TokenJSON
	resp, err := srv.CallJSON(ctx, &opts, nil, &jsonToken)
	if err != nil && authCode == "" {
		// if 2fa is enabled the first request is expected to fail. We will do another request with the 2fa code as an additional http header
		if resp != nil {
			if resp.Header.Get("X-JottaCloud-OTP") == "required; SMS" {
				return token, errAuthCodeRequired
			}
		}
	}

	token.AccessToken = jsonToken.AccessToken
	token.RefreshToken = jsonToken.RefreshToken
	token.TokenType = jsonToken.TokenType
	token.Expiry = time.Now().Add(time.Duration(jsonToken.ExpiresIn) * time.Second)
	return token, err
}

// doTokenAuth runs the actual token request for V2 authentication
func doTokenAuth(ctx context.Context, apiSrv *rest.Client, loginTokenBase64 string) (token oauth2.Token, tokenEndpoint string, err error) {
	loginTokenBytes, err := base64.RawURLEncoding.DecodeString(loginTokenBase64)
	if err != nil {
		return token, "", err
	}

	// decode login token
	var loginToken api.LoginToken
	decoder := json.NewDecoder(bytes.NewReader(loginTokenBytes))
	err = decoder.Decode(&loginToken)
	if err != nil {
		return token, "", err
	}

	// retrieve endpoint urls
	opts := rest.Opts{
		Method:  "GET",
		RootURL: loginToken.WellKnownLink,
	}
	var wellKnown api.WellKnown
	_, err = apiSrv.CallJSON(ctx, &opts, nil, &wellKnown)
	if err != nil {
		return token, "", err
	}

	// prepare out token request with username and password
	values := url.Values{}
	values.Set("client_id", defaultClientID)
	values.Set("grant_type", "password")
	values.Set("password", loginToken.AuthToken)
	values.Set("scope", "offline_access+openid")
	values.Set("username", loginToken.Username)
	values.Encode()
	opts = rest.Opts{
		Method:      "POST",
		RootURL:     wellKnown.TokenEndpoint,
		ContentType: "application/x-www-form-urlencoded",
		Body:        strings.NewReader(values.Encode()),
	}

	// do the first request
	var jsonToken api.TokenJSON
	_, err = apiSrv.CallJSON(ctx, &opts, nil, &jsonToken)
	if err != nil {
		return token, "", err
	}

	token.AccessToken = jsonToken.AccessToken
	token.RefreshToken = jsonToken.RefreshToken
	token.TokenType = jsonToken.TokenType
	token.Expiry = time.Now().Add(time.Duration(jsonToken.ExpiresIn) * time.Second)
	return token, wellKnown.TokenEndpoint, err
}

// getCustomerInfo queries general information about the account
func getCustomerInfo(ctx context.Context, apiSrv *rest.Client) (info *api.CustomerInfo, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   "account/v1/customer",
	}

	_, err = apiSrv.CallJSON(ctx, &opts, nil, &info)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get customer info")
	}

	return info, nil
}

// getDriveInfo queries general information about the account and the available devices and mountpoints.
func getDriveInfo(ctx context.Context, srv *rest.Client, username string) (info *api.DriveInfo, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   username,
	}

	_, err = srv.CallXML(ctx, &opts, nil, &info)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get drive info")
	}

	return info, nil
}

// getDeviceInfo queries Information about a jottacloud device
func getDeviceInfo(ctx context.Context, srv *rest.Client, path string) (info *api.JottaDevice, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   urlPathEscape(path),
	}

	_, err = srv.CallXML(ctx, &opts, nil, &info)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get device info")
	}

	return info, nil
}

// setEndpointURL generates the API endpoint URL
func (f *Fs) setEndpointURL() {
	if f.opt.Device == "" {
		f.opt.Device = defaultDevice
	}
	if f.opt.Mountpoint == "" {
		f.opt.Mountpoint = defaultMountpoint
	}
	f.endpointURL = path.Join(f.user, f.opt.Device, f.opt.Mountpoint)
}

// readMetaDataForPath reads the metadata from the path
func (f *Fs) readMetaDataForPath(ctx context.Context, path string) (info *api.JottaFile, err error) {
	opts := rest.Opts{
		Method: "GET",
		Path:   f.filePath(path),
	}
	var result api.JottaFile
	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})

	if apiErr, ok := err.(*api.Error); ok {
		// does not exist
		if apiErr.StatusCode == http.StatusNotFound {
			return nil, fs.ErrorObjectNotFound
		}
	}

	if err != nil {
		return nil, errors.Wrap(err, "read metadata failed")
	}
	if result.XMLName.Local != "file" {
		return nil, fs.ErrorNotAFile
	}
	return &result, nil
}

// errorHandler parses a non 2xx error response into an error
func errorHandler(resp *http.Response) error {
	// Decode error response
	errResponse := new(api.Error)
	err := rest.DecodeXML(resp, &errResponse)
	if err != nil {
		fs.Debugf(nil, "Couldn't decode error response: %v", err)
	}
	if errResponse.Message == "" {
		errResponse.Message = resp.Status
	}
	if errResponse.StatusCode == 0 {
		errResponse.StatusCode = resp.StatusCode
	}
	return errResponse
}

// Jottacloud wants '+' to be URL encoded even though the RFC states it's not reserved
func urlPathEscape(in string) string {
	return strings.Replace(rest.URLPathEscape(in), "+", "%2B", -1)
}

// filePathRaw returns an unescaped file path (f.root, file)
func (f *Fs) filePathRaw(file string) string {
	return path.Join(f.endpointURL, f.opt.Enc.FromStandardPath(path.Join(f.root, file)))
}

// filePath returns an escaped file path (f.root, file)
func (f *Fs) filePath(file string) string {
	return urlPathEscape(f.filePathRaw(file))
}

// Jottacloud requires the grant_type 'refresh_token' string
// to be uppercase and throws a 400 Bad Request if we use the
// lower case used by the oauth2 module
//
// This filter catches all refresh requests, reads the body,
// changes the case and then sends it on
func grantTypeFilter(req *http.Request) {
	if legacyTokenURL == req.URL.String() {
		// read the entire body
		refreshBody, err := ioutil.ReadAll(req.Body)
		if err != nil {
			return
		}
		_ = req.Body.Close()

		// make the refresh token upper case
		refreshBody = []byte(strings.Replace(string(refreshBody), "grant_type=refresh_token", "grant_type=REFRESH_TOKEN", 1))

		// set the new ReadCloser (with a dummy Close())
		req.Body = ioutil.NopCloser(bytes.NewReader(refreshBody))
	}
}

func getOAuthClient(ctx context.Context, name string, m configmap.Mapper) (oAuthClient *http.Client, ts *oauthutil.TokenSource, err error) {
	// Check config version
	var ver int
	version, ok := m.Get("configVersion")
	if ok {
		ver, err = strconv.Atoi(version)
		if err != nil {
			return nil, nil, errors.New("Failed to parse config version")
		}
		ok = (ver == configVersion) || (ver == legacyConfigVersion)
	}
	if !ok {
		return nil, nil, errors.New("Outdated config - please reconfigure this backend")
	}

	baseClient := fshttp.NewClient(ctx)
	oauthConfig := &oauth2.Config{
		Endpoint: oauth2.Endpoint{
			AuthURL:  defaultTokenURL,
			TokenURL: defaultTokenURL,
		},
	}
	if ver == configVersion {
		oauthConfig.ClientID = defaultClientID
		// if custom endpoints are set use them else stick with defaults
		if tokenURL, ok := m.Get(configTokenURL); ok {
			oauthConfig.Endpoint.TokenURL = tokenURL
			// jottacloud is weird. we need to use the tokenURL as authURL
			oauthConfig.Endpoint.AuthURL = tokenURL
		}
	} else if ver == legacyConfigVersion {
		clientID, ok := m.Get(configClientID)
		if !ok {
			clientID = legacyClientID
		}
		clientSecret, ok := m.Get(configClientSecret)
		if !ok {
			clientSecret = legacyEncryptedClientSecret
		}
		oauthConfig.ClientID = clientID
		oauthConfig.ClientSecret = obscure.MustReveal(clientSecret)

		oauthConfig.Endpoint.TokenURL = legacyTokenURL
		oauthConfig.Endpoint.AuthURL = legacyTokenURL

		// add the request filter to fix token refresh
		if do, ok := baseClient.Transport.(interface {
			SetRequestFilter(f func(req *http.Request))
		}); ok {
			do.SetRequestFilter(grantTypeFilter)
		} else {
			fs.Debugf(name+":", "Couldn't add request filter - uploads will fail")
		}
	}

	// Create OAuth Client
	oAuthClient, ts, err = oauthutil.NewClientWithBaseClient(ctx, name, m, oauthConfig, baseClient)
	if err != nil {
		return nil, nil, errors.Wrap(err, "Failed to configure Jottacloud oauth client")
	}
	return oAuthClient, ts, nil
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse config into Options struct
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	oAuthClient, ts, err := getOAuthClient(ctx, name, m)
	if err != nil {
		return nil, err
	}

	rootIsDir := strings.HasSuffix(root, "/")
	root = strings.Trim(root, "/")

	f := &Fs{
		name:   name,
		root:   root,
		opt:    *opt,
		srv:    rest.NewClient(oAuthClient).SetRoot(rootURL),
		apiSrv: rest.NewClient(oAuthClient).SetRoot(apiURL),
		pacer:  fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}
	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
		WriteMimeType:           false,
	}).Fill(ctx, f)
	f.srv.SetErrorHandler(errorHandler)
	if opt.TrashedOnly { // we cannot support showing Trashed Files when using ListR right now
		f.features.ListR = nil
	}

	// Renew the token in the background
	f.tokenRenewer = oauthutil.NewRenew(f.String(), ts, func() error {
		_, err := f.readMetaDataForPath(ctx, "")
		if err == fs.ErrorNotAFile {
			err = nil
		}
		return err
	})

	cust, err := getCustomerInfo(ctx, f.apiSrv)
	if err != nil {
		return nil, err
	}
	f.user = cust.Username
	f.setEndpointURL()

	if root != "" && !rootIsDir {
		// Check to see if the root actually an existing file
		remote := path.Base(root)
		f.root = path.Dir(root)
		if f.root == "." {
			f.root = ""
		}
		_, err := f.NewObject(context.TODO(), remote)
		if err != nil {
			if errors.Cause(err) == fs.ErrorObjectNotFound || errors.Cause(err) == fs.ErrorNotAFile {
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

// Return an Object from a path
//
// If it can't be found it returns the error fs.ErrorObjectNotFound.
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.JottaFile) (fs.Object, error) {
	o := &Object{
		fs:     f,
		remote: remote,
	}
	var err error
	if info != nil {
		// Set info
		err = o.setMetaData(info)
	} else {
		err = o.readMetaData(ctx, false) // reads info and meta, returning an error
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

// CreateDir makes a directory
func (f *Fs) CreateDir(ctx context.Context, path string) (jf *api.JottaFolder, err error) {
	// fs.Debugf(f, "CreateDir(%q, %q)\n", pathID, leaf)
	var resp *http.Response
	opts := rest.Opts{
		Method:     "POST",
		Path:       f.filePath(path),
		Parameters: url.Values{},
	}

	opts.Parameters.Set("mkDir", "true")

	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &jf)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		//fmt.Printf("...Error %v\n", err)
		return nil, err
	}
	// fmt.Printf("...Id %q\n", *info.Id)
	return jf, nil
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
	opts := rest.Opts{
		Method: "GET",
		Path:   f.filePath(dir),
	}

	var resp *http.Response
	var result api.JottaFolder
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})

	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				return nil, fs.ErrorDirNotFound
			}
		}
		return nil, errors.Wrap(err, "couldn't list files")
	}

	if bool(result.Deleted) && !f.opt.TrashedOnly {
		return nil, fs.ErrorDirNotFound
	}

	for i := range result.Folders {
		item := &result.Folders[i]
		if !f.opt.TrashedOnly && bool(item.Deleted) {
			continue
		}
		remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
		d := fs.NewDir(remote, time.Time(item.ModifiedAt))
		entries = append(entries, d)
	}

	for i := range result.Files {
		item := &result.Files[i]
		if f.opt.TrashedOnly {
			if !item.Deleted || item.State != "COMPLETED" {
				continue
			}
		} else {
			if item.Deleted || item.State != "COMPLETED" {
				continue
			}
		}
		remote := path.Join(dir, f.opt.Enc.ToStandardName(item.Name))
		o, err := f.newObjectWithInfo(ctx, remote, item)
		if err != nil {
			continue
		}
		entries = append(entries, o)
	}
	return entries, nil
}

// listFileDirFn is called from listFileDir to handle an object.
type listFileDirFn func(fs.DirEntry) error

// List the objects and directories into entries, from a
// special kind of JottaFolder representing a FileDirLis
func (f *Fs) listFileDir(ctx context.Context, remoteStartPath string, startFolder *api.JottaFolder, fn listFileDirFn) error {
	pathPrefix := "/" + f.filePathRaw("") // Non-escaped prefix of API paths to be cut off, to be left with the remote path including the remoteStartPath
	pathPrefixLength := len(pathPrefix)
	startPath := path.Join(pathPrefix, remoteStartPath) // Non-escaped API path up to and including remoteStartPath, to decide if it should be created as a new dir object
	startPathLength := len(startPath)
	for i := range startFolder.Folders {
		folder := &startFolder.Folders[i]
		if folder.Deleted {
			return nil
		}
		folderPath := f.opt.Enc.ToStandardPath(path.Join(folder.Path, folder.Name))
		folderPathLength := len(folderPath)
		var remoteDir string
		if folderPathLength > pathPrefixLength {
			remoteDir = folderPath[pathPrefixLength+1:]
			if folderPathLength > startPathLength {
				d := fs.NewDir(remoteDir, time.Time(folder.ModifiedAt))
				err := fn(d)
				if err != nil {
					return err
				}
			}
		}
		for i := range folder.Files {
			file := &folder.Files[i]
			if file.Deleted || file.State != "COMPLETED" {
				continue
			}
			remoteFile := path.Join(remoteDir, f.opt.Enc.ToStandardName(file.Name))
			o, err := f.newObjectWithInfo(ctx, remoteFile, file)
			if err != nil {
				return err
			}
			err = fn(o)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// ListR lists the objects and directories of the Fs starting
// from dir recursively into out.
//
// dir should be "" to start from the root, and should not
// have trailing slashes.
func (f *Fs) ListR(ctx context.Context, dir string, callback fs.ListRCallback) (err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       f.filePath(dir),
		Parameters: url.Values{},
	}
	opts.Parameters.Set("mode", "list")

	var resp *http.Response
	var result api.JottaFolder // Could be JottaFileDirList, but JottaFolder is close enough
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		if apiErr, ok := err.(*api.Error); ok {
			// does not exist
			if apiErr.StatusCode == http.StatusNotFound {
				return fs.ErrorDirNotFound
			}
		}
		return errors.Wrap(err, "couldn't list files")
	}
	list := walk.NewListRHelper(callback)
	err = f.listFileDir(ctx, dir, &result, func(entry fs.DirEntry) error {
		return list.Add(entry)
	})
	if err != nil {
		return err
	}
	return list.Flush()
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
	if f.opt.Device != "Jotta" {
		return nil, errors.New("upload not supported for devices other than Jotta")
	}
	o := f.createObject(src.Remote(), src.ModTime(ctx), src.Size())
	return o, o.Update(ctx, in, src, options...)
}

// mkParentDir makes the parent of the native path dirPath if
// necessary and any directories above that
func (f *Fs) mkParentDir(ctx context.Context, dirPath string) error {
	// defer log.Trace(dirPath, "")("")
	// chop off trailing / if it exists
	if strings.HasSuffix(dirPath, "/") {
		dirPath = dirPath[:len(dirPath)-1]
	}
	parent := path.Dir(dirPath)
	if parent == "." {
		parent = ""
	}
	return f.Mkdir(ctx, parent)
}

// Mkdir creates the container if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.CreateDir(ctx, dir)
	return err
}

// purgeCheck removes the root directory, if check is set then it
// refuses to do so if it has anything in
func (f *Fs) purgeCheck(ctx context.Context, dir string, check bool) (err error) {
	root := path.Join(f.root, dir)
	if root == "" {
		return errors.New("can't purge root directory")
	}

	// check that the directory exists
	entries, err := f.List(ctx, dir)
	if err != nil {
		return err
	}

	if check {
		if len(entries) != 0 {
			return fs.ErrorDirectoryNotEmpty
		}
	}

	opts := rest.Opts{
		Method:     "POST",
		Path:       f.filePath(dir),
		Parameters: url.Values{},
		NoResponse: true,
	}

	if f.opt.HardDelete {
		opts.Parameters.Set("rmDir", "true")
	} else {
		opts.Parameters.Set("dlDir", "true")
	}

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return errors.Wrap(err, "couldn't purge directory")
	}

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
	return time.Second
}

// Purge deletes all the files and the container
func (f *Fs) Purge(ctx context.Context, dir string) error {
	return f.purgeCheck(ctx, dir, false)
}

// copyOrMoves copies or moves directories or files depending on the method parameter
func (f *Fs) copyOrMove(ctx context.Context, method, src, dest string) (info *api.JottaFile, err error) {
	opts := rest.Opts{
		Method:     "POST",
		Path:       src,
		Parameters: url.Values{},
	}

	opts.Parameters.Set(method, "/"+path.Join(f.endpointURL, f.opt.Enc.FromStandardPath(path.Join(f.root, dest))))

	var resp *http.Response
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &info)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return info, nil
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't copy - not same remote type")
		return nil, fs.ErrorCantMove
	}

	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, err
	}
	info, err := f.copyOrMove(ctx, "cp", srcObj.filePath(), remote)

	if err != nil {
		return nil, errors.Wrap(err, "couldn't copy file")
	}

	return f.newObjectWithInfo(ctx, remote, info)
	//return f.newObjectWithInfo(remote, &result)
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
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	err := f.mkParentDir(ctx, remote)
	if err != nil {
		return nil, err
	}
	info, err := f.copyOrMove(ctx, "mv", srcObj.filePath(), remote)

	if err != nil {
		return nil, errors.Wrap(err, "couldn't move file")
	}

	return f.newObjectWithInfo(ctx, remote, info)
	//return f.newObjectWithInfo(remote, result)
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
	srcPath := path.Join(srcFs.root, srcRemote)
	dstPath := path.Join(f.root, dstRemote)

	// Refuse to move to or from the root
	if srcPath == "" || dstPath == "" {
		fs.Debugf(src, "DirMove error: Can't move root")
		return errors.New("can't move root directory")
	}
	//fmt.Printf("Move src: %s (FullPath %s), dst: %s (FullPath: %s)\n", srcRemote, srcPath, dstRemote, dstPath)

	var err error
	_, err = f.List(ctx, dstRemote)
	if err == fs.ErrorDirNotFound {
		// OK
	} else if err != nil {
		return err
	} else {
		return fs.ErrorDirExists
	}

	_, err = f.copyOrMove(ctx, "mvDir", path.Join(f.endpointURL, f.opt.Enc.FromStandardPath(srcPath))+"/", dstRemote)

	if err != nil {
		return errors.Wrap(err, "couldn't move directory")
	}
	return nil
}

// PublicLink generates a public link to the remote path (usually readable by anyone)
func (f *Fs) PublicLink(ctx context.Context, remote string, expire fs.Duration, unlink bool) (link string, err error) {
	opts := rest.Opts{
		Method:     "GET",
		Path:       f.filePath(remote),
		Parameters: url.Values{},
	}

	if unlink {
		opts.Parameters.Set("mode", "disableShare")
	} else {
		opts.Parameters.Set("mode", "enableShare")
	}

	var resp *http.Response
	var result api.JottaFile
	err = f.pacer.Call(func() (bool, error) {
		resp, err = f.srv.CallXML(ctx, &opts, nil, &result)
		return shouldRetry(ctx, resp, err)
	})

	if apiErr, ok := err.(*api.Error); ok {
		// does not exist
		if apiErr.StatusCode == http.StatusNotFound {
			return "", fs.ErrorObjectNotFound
		}
	}
	if err != nil {
		if unlink {
			return "", errors.Wrap(err, "couldn't remove public link")
		}
		return "", errors.Wrap(err, "couldn't create public link")
	}
	if unlink {
		if result.PublicSharePath != "" {
			return "", errors.Errorf("couldn't remove public link - %q", result.PublicSharePath)
		}
		return "", nil
	}
	if result.PublicSharePath == "" {
		return "", errors.New("couldn't create public link - no link path received")
	}
	return joinPath(baseURL, result.PublicSharePath), nil
}

// About gets quota information
func (f *Fs) About(ctx context.Context) (*fs.Usage, error) {
	info, err := getDriveInfo(ctx, f.srv, f.user)
	if err != nil {
		return nil, err
	}

	usage := &fs.Usage{
		Used: fs.NewUsageValue(info.Usage),
	}
	if info.Capacity > 0 {
		usage.Total = fs.NewUsageValue(info.Capacity)
		usage.Free = fs.NewUsageValue(info.Capacity - info.Usage)
	}
	return usage, nil
}

// CleanUp empties the trash
func (f *Fs) CleanUp(ctx context.Context) error {
	opts := rest.Opts{
		Method: "POST",
		Path:   "files/v1/purge_trash",
	}

	var info api.TrashResponse
	_, err := f.apiSrv.CallJSON(ctx, &opts, nil, &info)
	if err != nil {
		return errors.Wrap(err, "couldn't empty trash")
	}

	return nil
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.MD5)
}

// ---------------------------------------------

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

// filePath returns an escaped file path (f.root, remote)
func (o *Object) filePath() string {
	return o.fs.filePath(o.remote)
}

// Hash returns the MD5 of an object returning a lowercase hex string
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	if t != hash.MD5 {
		return "", hash.ErrUnsupported
	}
	return o.md5, nil
}

// Size returns the size of an object in bytes
func (o *Object) Size() int64 {
	ctx := context.TODO()
	err := o.readMetaData(ctx, false)
	if err != nil {
		fs.Logf(o, "Failed to read metadata: %v", err)
		return 0
	}
	return o.size
}

// MimeType of an Object if known, "" otherwise
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// setMetaData sets the metadata from info
func (o *Object) setMetaData(info *api.JottaFile) (err error) {
	o.hasMetaData = true
	o.size = info.Size
	o.md5 = info.MD5
	o.mimeType = info.MimeType
	o.modTime = time.Time(info.ModifiedAt)
	return nil
}

// readMetaData reads and updates the metadata for an object
func (o *Object) readMetaData(ctx context.Context, force bool) (err error) {
	if o.hasMetaData && !force {
		return nil
	}
	info, err := o.fs.readMetaDataForPath(ctx, o.remote)
	if err != nil {
		return err
	}
	if bool(info.Deleted) && !o.fs.opt.TrashedOnly {
		return fs.ErrorObjectNotFound
	}
	return o.setMetaData(info)
}

// ModTime returns the modification time of the object
//
// It attempts to read the objects mtime and if that isn't present the
// LastModified returned in the http headers
func (o *Object) ModTime(ctx context.Context) time.Time {
	err := o.readMetaData(ctx, false)
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
	fs.FixRangeOption(options, o.size)
	var resp *http.Response
	opts := rest.Opts{
		Method:     "GET",
		Path:       o.filePath(),
		Parameters: url.Values{},
		Options:    options,
	}

	opts.Parameters.Set("mode", "bin")

	err = o.fs.pacer.Call(func() (bool, error) {
		resp, err = o.fs.srv.Call(ctx, &opts)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return nil, err
	}
	return resp.Body, err
}

// Read the md5 of in returning a reader which will read the same contents
//
// The cleanup function should be called when out is finished with
// regardless of whether this function returned an error or not.
func readMD5(in io.Reader, size, threshold int64) (md5sum string, out io.Reader, cleanup func(), err error) {
	// we need an MD5
	md5Hasher := md5.New()
	// use the teeReader to write to the local file AND calculate the MD5 while doing so
	teeReader := io.TeeReader(in, md5Hasher)

	// nothing to clean up by default
	cleanup = func() {}

	// don't cache small files on disk to reduce wear of the disk
	if size > threshold {
		var tempFile *os.File

		// create the cache file
		tempFile, err = ioutil.TempFile("", cachePrefix)
		if err != nil {
			return
		}

		_ = os.Remove(tempFile.Name()) // Delete the file - may not work on Windows

		// clean up the file after we are done downloading
		cleanup = func() {
			// the file should normally already be close, but just to make sure
			_ = tempFile.Close()
			_ = os.Remove(tempFile.Name()) // delete the cache file after we are done - may be deleted already
		}

		// copy the ENTIRE file to disc and calculate the MD5 in the process
		if _, err = io.Copy(tempFile, teeReader); err != nil {
			return
		}
		// jump to the start of the local file so we can pass it along
		if _, err = tempFile.Seek(0, 0); err != nil {
			return
		}

		// replace the already read source with a reader of our cached file
		out = tempFile
	} else {
		// that's a small file, just read it into memory
		var inData []byte
		inData, err = ioutil.ReadAll(teeReader)
		if err != nil {
			return
		}

		// set the reader to our read memory block
		out = bytes.NewReader(inData)
	}
	return hex.EncodeToString(md5Hasher.Sum(nil)), out, cleanup, nil
}

// Update the object with the contents of the io.Reader, modTime and size
//
// If existing is set then it updates the object rather than creating a new one
//
// The new object may have been created if an error is returned
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (err error) {
	o.fs.tokenRenewer.Start()
	defer o.fs.tokenRenewer.Stop()
	size := src.Size()
	md5String, err := src.Hash(ctx, hash.MD5)
	if err != nil || md5String == "" {
		// unwrap the accounting from the input, we use wrap to put it
		// back on after the buffering
		var wrap accounting.WrapFn
		in, wrap = accounting.UnWrap(in)
		var cleanup func()
		md5String, in, cleanup, err = readMD5(in, size, int64(o.fs.opt.MD5MemoryThreshold))
		defer cleanup()
		if err != nil {
			return errors.Wrap(err, "failed to calculate MD5")
		}
		// Wrap the accounting back onto the stream
		in = wrap(in)
	}

	// use the api to allocate the file first and get resume / deduplication info
	var resp *http.Response
	opts := rest.Opts{
		Method:       "POST",
		Path:         "files/v1/allocate",
		Options:      options,
		ExtraHeaders: make(map[string]string),
	}
	fileDate := api.Time(src.ModTime(ctx)).APIString()

	// the allocate request
	var request = api.AllocateFileRequest{
		Bytes:    size,
		Created:  fileDate,
		Modified: fileDate,
		Md5:      md5String,
		Path:     path.Join(o.fs.opt.Mountpoint, o.fs.opt.Enc.FromStandardPath(path.Join(o.fs.root, o.remote))),
	}

	// send it
	var response api.AllocateFileResponse
	err = o.fs.pacer.CallNoRetry(func() (bool, error) {
		resp, err = o.fs.apiSrv.CallJSON(ctx, &opts, &request, &response)
		return shouldRetry(ctx, resp, err)
	})
	if err != nil {
		return err
	}

	// If the file state is INCOMPLETE and CORRUPT, try to upload a then
	if response.State != "COMPLETED" {
		// how much do we still have to upload?
		remainingBytes := size - response.ResumePos
		opts = rest.Opts{
			Method:        "POST",
			RootURL:       response.UploadURL,
			ContentLength: &remainingBytes,
			ContentType:   "application/octet-stream",
			Body:          in,
			ExtraHeaders:  make(map[string]string),
		}
		if response.ResumePos != 0 {
			opts.ExtraHeaders["Range"] = "bytes=" + strconv.FormatInt(response.ResumePos, 10) + "-" + strconv.FormatInt(size-1, 10)
		}

		// copy the already uploaded bytes into the trash :)
		var result api.UploadResponse
		_, err = io.CopyN(ioutil.Discard, in, response.ResumePos)
		if err != nil {
			return err
		}

		// send the remaining bytes
		resp, err = o.fs.apiSrv.CallJSON(ctx, &opts, nil, &result)
		if err != nil {
			return err
		}

		// finally update the meta data
		o.hasMetaData = true
		o.size = result.Bytes
		o.md5 = result.Md5
		o.modTime = time.Unix(result.Modified/1000, 0)
	} else {
		// If the file state is COMPLETE we don't need to upload it because the file was already found but we still ned to update our metadata
		return o.readMetaData(ctx, true)
	}

	return nil
}

// Remove an object
func (o *Object) Remove(ctx context.Context) error {
	opts := rest.Opts{
		Method:     "POST",
		Path:       o.filePath(),
		Parameters: url.Values{},
		NoResponse: true,
	}

	if o.fs.opt.HardDelete {
		opts.Parameters.Set("rm", "true")
	} else {
		opts.Parameters.Set("dl", "true")
	}

	return o.fs.pacer.Call(func() (bool, error) {
		resp, err := o.fs.srv.CallXML(ctx, &opts, nil, nil)
		return shouldRetry(ctx, resp, err)
	})
}

// Check the interfaces are satisfied
var (
	_ fs.Fs           = (*Fs)(nil)
	_ fs.Purger       = (*Fs)(nil)
	_ fs.Copier       = (*Fs)(nil)
	_ fs.Mover        = (*Fs)(nil)
	_ fs.DirMover     = (*Fs)(nil)
	_ fs.ListRer      = (*Fs)(nil)
	_ fs.PublicLinker = (*Fs)(nil)
	_ fs.Abouter      = (*Fs)(nil)
	_ fs.CleanUpper   = (*Fs)(nil)
	_ fs.Object       = (*Object)(nil)
	_ fs.MimeTyper    = (*Object)(nil)
)
