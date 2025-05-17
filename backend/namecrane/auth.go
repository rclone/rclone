package namecrane

import (
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

	if response.Token, ok = c.m.Get(accessTokenKey); !ok {
		fs.Debugf(c, "Token not found in config mapper")
		return nil, nil
	} else {
		response.Token, err = obscure.Reveal(response.Token)

		if err != nil {
			return nil, err
		}
	}

	if tokenExpiration, ok := c.m.Get(accessTokenExpireKey); ok {
		response.TokenExpiration, err = time.Parse(time.RFC3339, tokenExpiration)

		if err != nil {
			return nil, err
		}
	} else {
		fs.Debugf(c, "Token expiration not found in config mapper")
		return nil, nil
	}

	if response.RefreshToken, ok = c.m.Get(refreshTokenKey); !ok {
		fs.Debugf(c, "Refresh token not found in config mapper")
		return nil, nil
	} else {
		response.RefreshToken, err = obscure.Reveal(response.RefreshToken)

		if err != nil {
			return nil, err
		}
	}

	if refreshTokenExpiration, ok := c.m.Get(refreshTokenExpireKey); ok {
		var err error
		response.RefreshTokenExpiration, err = time.Parse(time.RFC3339, refreshTokenExpiration)

		if err != nil {
			return nil, err
		}
	} else {
		fs.Debugf(c, "Refresh token expiration not found in config mapper")
		return nil, nil
	}

	fs.Debugf(c, "All information found and filled")

	c.lastResponse = &response

	return c.lastResponse, nil
}
