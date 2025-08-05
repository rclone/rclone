//go:build !plan9 && !solaris

// Package iclouddrive provides access to iCloud Drive and Photos
package iclouddrive

import (
	"context"
	"errors"
	"fmt"

	"github.com/rclone/rclone/backend/iclouddrive/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/lib/encoder"
)

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

	switch config.State {
	case "":
		icloud, err := api.New(appleid, password, trustToken, clientID, cookies, nil)
		if err != nil {
			return nil, err
		}
		if err := icloud.Authenticate(ctx); err != nil {
			return nil, err
		}
		m.Set(configCookies, icloud.Session.GetCookieString())
		if icloud.Session.Requires2FA() {
			return fs.ConfigInput("2fa_do", "config_2fa", "Two-factor authentication: please enter your 2FA code")
		}
		return nil, nil
	case "2fa_do":
		code := config.Result
		if code == "" {
			return fs.ConfigError("authenticate", "2FA codes can't be blank")
		}

		icloud, err := api.New(appleid, password, trustToken, clientID, cookies, nil)
		if err != nil {
			return nil, err
		}
		if err := icloud.SignIn(ctx); err != nil {
			return nil, err
		}

		if err := icloud.Session.Validate2FACode(ctx, code); err != nil {
			return nil, err
		}

		m.Set(configTrustToken, icloud.Session.TrustToken)
		m.Set(configCookies, icloud.Session.GetCookieString())
		return nil, nil

	case "2fa_error":
		if config.Result == "true" {
			return fs.ConfigGoto("2fa")
		}
		return nil, errors.New("2fa authentication failed")
	}
	return nil, fmt.Errorf("unknown state %q", config.State)
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
