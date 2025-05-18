package namecrane

import (
	"fmt"
	"github.com/namecrane/hoist"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/obscure"
	"time"
)

const (
	accessTokenKey        = "access_token"
	accessTokenExpireKey  = "access_token_expires"
	refreshTokenKey       = "refresh_token"
	refreshTokenExpireKey = "refresh_token_expires"
)

type ConfigMapperStore struct {
	m            configmap.Mapper
	lastResponse *hoist.AuthResponse
}

func (c *ConfigMapperStore) Set(username string, response hoist.AuthResponse) {
	c.lastResponse = &response

	fs.Debugf(c, "Saving values to config mapper")

	c.m.Set(accessTokenKey, obscure.MustObscure(response.Token))
	c.m.Set(accessTokenExpireKey, response.TokenExpiration.Format(time.RFC3339))
	c.m.Set(refreshTokenKey, obscure.MustObscure(response.RefreshToken))
	c.m.Set(refreshTokenExpireKey, response.RefreshTokenExpiration.Format(time.RFC3339))
}

func (c *ConfigMapperStore) Get(username string) (*hoist.AuthResponse, error) {
	if c.lastResponse != nil {
		return c.lastResponse, nil
	}

	fs.Debugf(c, "Filling last response value from config mapper")

	var response hoist.AuthResponse
	var ok bool
	var err error

	if response.Token, ok = c.m.Get(accessTokenKey); ok && response.Token != "" {
		fs.Debugf(c, "Found token in config mapper: %s", response.Token)
		if response.Token != "" {
			response.Token, err = obscure.Reveal(response.Token)

			if err != nil {
				return nil, fmt.Errorf("token reveal failed: %v", err)
			}
		}
	} else {
		fs.Debugf(c, "Token not found in config mapper")
	}

	if tokenExpiration, ok := c.m.Get(accessTokenExpireKey); ok && tokenExpiration != "" {
		response.TokenExpiration, err = time.Parse(time.RFC3339, tokenExpiration)

		if err != nil {
			return nil, err
		}

		fs.Debugf(c, "Token expiration is %v", response.TokenExpiration)
	} else {
		fs.Debugf(c, "Token expiration not found in config mapper")
	}

	if response.RefreshToken, ok = c.m.Get(refreshTokenKey); ok && response.RefreshToken != "" {
		response.RefreshToken, err = obscure.Reveal(response.RefreshToken)

		if err != nil {
			return nil, err
		}
	} else {
		fs.Debugf(c, "Refresh token not found in config mapper")
	}

	if refreshTokenExpiration, ok := c.m.Get(refreshTokenExpireKey); ok && refreshTokenExpiration != "" {
		var err error
		response.RefreshTokenExpiration, err = time.Parse(time.RFC3339, refreshTokenExpiration)

		if err != nil {
			return nil, err
		}

		fs.Debugf(c, "Refresh token expiration is %v", response.RefreshTokenExpiration)
	} else {
		fs.Debugf(c, "Refresh token expiration not found in config mapper")
	}

	fs.Debugf(c, "All information filled")

	c.lastResponse = &response

	return c.lastResponse, nil
}
