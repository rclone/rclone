//go:build !plan9 && !solaris

// Package iclouddrive provides access to iCloud Drive and Photos
package iclouddrive

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/encoder"
)

const configAuthSession = "_auth_session"

// configAuthState holds session fields that must be preserved across
// Config() state machine steps (push 2FA and SMS both need it)
type configAuthState struct {
	Scnt           string     `json:"s"`
	SessionID      string     `json:"i"`
	AuthAttributes string     `json:"a"`
	FrameID        string     `json:"f"`
	SessionToken   string     `json:"t"`
	ClientID       string     `json:"c"`
	AccountCountry string     `json:"ac"`
	Phones         []smsPhone `json:"p,omitempty"`
}

type smsPhone struct {
	ID   int    `json:"id"`
	Num  string `json:"n"`
	Mode string `json:"m"`
}

func saveAuthSession(m configmap.Mapper, s *api.Session, phones []smsPhone) {
	st := configAuthState{
		Scnt:           s.Scnt,
		SessionID:      s.SessionID,
		AuthAttributes: s.AuthAttributes,
		FrameID:        s.FrameID,
		SessionToken:   s.SessionToken,
		ClientID:       s.ClientID,
		AccountCountry: s.AccountCountry,
		Phones:         phones,
	}
	data, err := json.Marshal(st)
	if err != nil {
		fs.Debugf(nil, "iclouddrive: failed to marshal auth session: %v", err)
		return
	}
	m.Set(configAuthSession, base64.StdEncoding.EncodeToString(data))
}

func loadAuthSession(m configmap.Mapper) (*configAuthState, error) {
	raw, _ := m.Get(configAuthSession)
	if raw == "" {
		return nil, errors.New("auth session state lost, please reconfigure")
	}
	data, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, fmt.Errorf("corrupt auth session state: %w", err)
	}
	var st configAuthState
	if err := json.Unmarshal(data, &st); err != nil {
		return nil, fmt.Errorf("invalid auth session state: %w", err)
	}
	return &st, nil
}

func restoreAuthSession(icloud *api.Client, st *configAuthState) {
	icloud.Session.Scnt = st.Scnt
	icloud.Session.SessionID = st.SessionID
	icloud.Session.AuthAttributes = st.AuthAttributes
	icloud.Session.FrameID = st.FrameID
	icloud.Session.SessionToken = st.SessionToken
	icloud.Session.ClientID = st.ClientID
	icloud.Session.AccountCountry = st.AccountCountry
}

// resumeConfigClient recreates an API client and restores session state saved
// from a previous Config() step. Used across 2FA states to avoid re-authenticating
func resumeConfigClient(m configmap.Mapper, appleid, password, trustToken, clientID string, cookies []*http.Cookie) (*api.Client, *configAuthState, error) {
	st, err := loadAuthSession(m)
	if err != nil {
		return nil, nil, err
	}
	icloud, err := api.New(appleid, password, trustToken, clientID, cookies, nil, "_config")
	if err != nil {
		return nil, nil, err
	}
	restoreAuthSession(icloud, st)
	return icloud, st, nil
}

// saveAuthCredentials persists trust token, cookies, and clears session state
// after successful 2FA validation
func saveAuthCredentials(m configmap.Mapper, icloud *api.Client, name string) {
	m.Set(configTrustToken, icloud.Session.TrustToken)
	m.Set(configCookies, icloud.Session.GetCookieString())
	m.Set(configAuthSession, "")
	api.ClearCacheDir(name)
}

// triggerSMSFlow handles phone selection and SMS triggering
// Single phone: triggers SMS immediately, returns code prompt
// Multiple phones: saves session + phone list, returns phone picker
func triggerSMSFlow(ctx context.Context, icloud *api.Client, phones []api.TrustedPhoneNumber, m configmap.Mapper) (*fs.ConfigOut, error) {
	smsPhones := make([]smsPhone, len(phones))
	for i, p := range phones {
		mode := p.PushMode
		if mode == "" {
			mode = "sms"
		}
		smsPhones[i] = smsPhone{ID: p.ID, Num: p.NumberWithDialCode, Mode: mode}
	}

	if len(phones) == 1 {
		p := smsPhones[0]
		if err := icloud.Session.RequestSMSCode(ctx, p.ID, p.Mode); err != nil {
			return nil, fmt.Errorf("failed to send SMS code: %w", err)
		}
		saveAuthSession(m, icloud.Session, nil)
		nextState := fmt.Sprintf("2fa_sms_%d_%s", p.ID, p.Mode)
		return fs.ConfigInput(nextState, "config_2fa_sms", fmt.Sprintf("Enter the verification code sent to %s", p.Num))
	}

	// Multiple phones - save session and present picker
	saveAuthSession(m, icloud.Session, smsPhones)
	items := make([]fs.OptionExample, len(smsPhones))
	for i, p := range smsPhones {
		items[i] = fs.OptionExample{
			Value: fmt.Sprintf("%d_%s", p.ID, p.Mode),
			Help:  p.Num,
		}
	}
	return fs.ConfigChooseExclusiveFixed("2fa_sms_select", "config_2fa_phone", "Select phone number for SMS verification", items)
}

const (
	configService = "service"

	// Service types
	serviceDrive  = "drive"
	servicePhotos = "photos"
)

// ServiceOptions defines the configuration for service selection
type ServiceOptions struct {
	Service string `config:"service"`
}

// Register with rclone
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "iclouddrive",
		Description: "iCloud Drive and Photos",
		Config:      Config,
		NewFs:       NewServiceFs,
		MetadataInfo: &fs.MetadataInfo{
			System: map[string]fs.MetadataHelp{
				"width": {
					Help:     "Image width in pixels",
					Type:     "int",
					ReadOnly: true,
				},
				"height": {
					Help:     "Image height in pixels",
					Type:     "int",
					ReadOnly: true,
				},
				"added-time": {
					Help:     "Time the item was added to the iCloud library",
					Type:     "RFC 3339",
					Example:  "2006-01-02T15:04:05Z",
					ReadOnly: true,
				},
				"favorite": {
					Help:     "Whether the item is marked as favorite",
					Type:     "bool",
					ReadOnly: true,
				},
				"hidden": {
					Help:     "Whether the item is hidden",
					Type:     "bool",
					ReadOnly: true,
				},
			},
			Help: "Metadata is read-only and available for the Photos service only.",
		},
		Options: []fs.Option{{
			Name:     configService,
			Help:     "iCloud service to use.",
			Required: true,
			Default:  serviceDrive,
			Examples: []fs.OptionExample{{
				Value: serviceDrive,
				Help:  "iCloud Drive",
			}, {
				Value: servicePhotos,
				Help:  "iCloud Photos",
			}},
		}, {
			Name:      configAppleID,
			Help:      "Apple ID.",
			Required:  true,
			Sensitive: true,
		}, {
			Name:       configPassword,
			Help:       "Password.",
			Required:   true,
			IsPassword: true,
			Sensitive:  true,
		}, {
			Name:       configTrustToken,
			Help:       "Trust token for session authentication.",
			IsPassword: false,
			Required:   false,
			Sensitive:  true,
			Hide:       fs.OptionHideBoth,
		}, {
			Name:      configCookies,
			Help:      "Session cookies.",
			Required:  false,
			Advanced:  false,
			Sensitive: true,
			Hide:      fs.OptionHideBoth,
		}, {
			Name:     configClientID,
			Help:     "Client ID for iCloud API access.",
			Required: false,
			Advanced: true,
			Default:  "d39ba9916b7251055b22c7f910e2ea796ee65e98b2ddecea8f5dde8d9d1a815d",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Config handles the authentication and configuration flow
func Config(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
	var err error
	appleid, _ := m.Get(configAppleID)
	if appleid == "" {
		return nil, errors.New("an Apple ID is required")
	}

	password, _ := m.Get(configPassword)
	if password != "" {
		password, err = obscure.Reveal(password)
		if err != nil {
			return nil, err
		}
	}

	trustToken, _ := m.Get(configTrustToken)
	cookieRaw, _ := m.Get(configCookies)
	clientID, _ := m.Get(configClientID)
	cookies := ReadCookies(cookieRaw)

	switch {
	case config.State == "":
		// Force fresh SRP authentication - ignore stale trust token and cookies
		// so that reconnect always prompts for 2FA
		m.Set(configAuthSession, "")
		icloud, err := api.New(appleid, password, "", clientID, nil, nil, "_config")
		if err != nil {
			return nil, err
		}
		if err := icloud.Authenticate(ctx); err != nil {
			return nil, err
		}
		m.Set(configCookies, icloud.Session.GetCookieString())
		if icloud.Session.Requires2FA() {
			// Check if user has no trusted devices - auto-trigger SMS if so
			authState, err := icloud.Session.GetAuthState(ctx)
			if err == nil && authState.NoTrustedDevices && len(authState.TrustedPhoneNumbers) > 0 {
				return triggerSMSFlow(ctx, icloud, authState.TrustedPhoneNumbers, m)
			}
			// Explicitly request push to trusted devices - required for iOS 26.4+
			// where the SRP 409 no longer auto-pushes. GET /appleauth/auth above
			// may also trigger a push (cosmetic double on pre-26.4, harmless)
			if err := icloud.Session.RequestPushNotification(ctx); err != nil {
				fs.Debugf(nil, "iclouddrive: push notification request failed (SMS fallback available): %v", err)
			} else {
				fs.Debugf(nil, "iclouddrive: push notification requested to trusted devices")
			}
			// Save session state so 2fa_do can validate without re-authenticating
			// Push codes are account-scoped so session reuse is not strictly required,
			// but it avoids a redundant SRP roundtrip and a second push on pre-26.4
			saveAuthSession(m, icloud.Session, nil)
			return fs.ConfigInput("2fa_do", "config_2fa", "Two-factor authentication: enter your 2FA code or type 'sms' for a text message")
		}
		// Auth succeeded without 2FA - save updated credentials and clear stale cache
		m.Set(configTrustToken, icloud.Session.TrustToken)
		api.ClearCacheDir(name)
		return nil, nil

	case config.State == "2fa_do":
		code := config.Result
		if code == "" {
			return fs.ConfigError("authenticate", "2FA codes can't be blank")
		}

		// Restore session from initial sign-in instead of re-authenticating
		// This avoids a redundant SRP roundtrip and extra push on pre-26.4
		icloud, _, err := resumeConfigClient(m, appleid, password, trustToken, clientID, cookies)
		if err != nil {
			return nil, err
		}

		if strings.EqualFold(code, "sms") {
			authState, err := icloud.Session.GetAuthState(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to get trusted phone numbers: %w", err)
			}
			if len(authState.TrustedPhoneNumbers) == 0 {
				return nil, errors.New("no trusted phone numbers on this account")
			}
			return triggerSMSFlow(ctx, icloud, authState.TrustedPhoneNumbers, m)
		}

		if err := icloud.Session.Validate2FACode(ctx, code); err != nil {
			return nil, err
		}
		saveAuthCredentials(m, icloud, name)
		return nil, nil

	case config.State == "2fa_sms_select":
		// User selected a phone from ConfigChooseExclusiveFixed (value: "ID_mode")
		idStr, mode, _ := strings.Cut(config.Result, "_")
		phoneID, err := strconv.Atoi(idStr)
		if err != nil {
			m.Set(configAuthSession, "")
			return nil, fmt.Errorf("invalid phone selection %q", config.Result)
		}
		if mode == "" {
			mode = "sms"
		}
		icloud, smsState, err := resumeConfigClient(m, appleid, password, trustToken, clientID, cookies)
		if err != nil {
			return nil, err
		}
		// Find the selected phone's display number
		var phoneNum string
		for _, p := range smsState.Phones {
			if p.ID == phoneID {
				phoneNum = p.Num
				break
			}
		}

		if err := icloud.Session.RequestSMSCode(ctx, phoneID, mode); err != nil {
			m.Set(configAuthSession, "")
			return nil, fmt.Errorf("failed to send SMS code: %w", err)
		}
		saveAuthSession(m, icloud.Session, nil)
		nextState := fmt.Sprintf("2fa_sms_%d_%s", phoneID, mode)
		return fs.ConfigInput(nextState, "config_2fa_sms", fmt.Sprintf("Enter the verification code sent to %s", phoneNum))

	case strings.HasPrefix(config.State, "2fa_sms_"):
		code := config.Result
		if code == "" {
			return fs.ConfigError("authenticate", "SMS code can't be blank")
		}
		// State encodes phone ID and mode: "2fa_sms_<ID>_<mode>"
		suffix := strings.TrimPrefix(config.State, "2fa_sms_")
		idStr, mode, _ := strings.Cut(suffix, "_")
		phoneID, err := strconv.Atoi(idStr)
		if err != nil {
			return nil, fmt.Errorf("invalid phone ID in state %q: %w", config.State, err)
		}
		if mode == "" {
			mode = "sms"
		}

		icloud, _, err := resumeConfigClient(m, appleid, password, trustToken, clientID, cookies)
		if err != nil {
			return nil, err
		}

		if err := icloud.Session.ValidateSMSCode(ctx, code, phoneID, mode); err != nil {
			m.Set(configAuthSession, "")
			return nil, err
		}
		saveAuthCredentials(m, icloud, name)
		return nil, nil

	default:
		return nil, fmt.Errorf("unknown state %q", config.State)
	}
}

// newICloudClient parses options, authenticates, and returns a ready client
// Shared between NewFs (Drive) and NewFsPhotos (Photos) to avoid duplication
func newICloudClient(ctx context.Context, name string, m configmap.Mapper) (*api.Client, *Options, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, nil, err
	}

	if opt.Password != "" {
		opt.Password, err = obscure.Reveal(opt.Password)
		if err != nil {
			return nil, nil, fmt.Errorf("couldn't decrypt user password: %w", err)
		}
	}

	if opt.TrustToken == "" {
		return nil, nil, fmt.Errorf("missing icloud trust token: try refreshing it with \"rclone config reconnect %s:\"", name)
	}

	cookies := ReadCookies(opt.Cookies)

	callback := func(session *api.Session) {
		m.Set(configCookies, session.GetCookieString())
	}

	icloud, err := api.New(
		opt.AppleID,
		opt.Password,
		opt.TrustToken,
		opt.ClientID,
		cookies,
		callback,
		name,
	)
	if err != nil {
		return nil, nil, err
	}

	if err := icloud.Authenticate(ctx); err != nil {
		return nil, nil, err
	}

	if icloud.Session.Requires2FA() {
		return nil, nil, errors.New("trust token expired, please reauth")
	}

	return icloud, opt, nil
}

// disconnectClient clears authentication state and removes disk caches
// Shared between Drive Fs and Photos Fs Disconnect() implementations
func disconnectClient(m configmap.Mapper, icloud *api.Client) error {
	m.Set(configTrustToken, "")
	m.Set(configCookies, "")
	m.Set(configAuthSession, "")
	return os.RemoveAll(icloud.CacheDir())
}

// NewServiceFs creates a filesystem instance for the selected service
func NewServiceFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	// Parse the service selection
	opt := new(ServiceOptions)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	// Set default service if not specified
	if opt.Service == "" {
		opt.Service = serviceDrive
	}

	// Route to the appropriate backend
	switch opt.Service {
	case serviceDrive:
		// Create Drive filesystem
		return NewFs(ctx, name, root, m)
	case servicePhotos:
		// Create Photos filesystem
		return NewFsPhotos(ctx, name, root, m)
	default:
		return nil, fmt.Errorf("invalid service selection: %s (must be 'drive' or 'photos')", opt.Service)
	}
}
