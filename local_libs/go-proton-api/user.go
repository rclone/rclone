package proton

import (
	"context"
	"encoding/base64"

	"github.com/ProtonMail/go-srp"
	"github.com/go-resty/resty/v2"
)

func (c *Client) GetUser(ctx context.Context) (User, error) {
	var res struct {
		User User
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/core/v4/users")
	}); err != nil {
		return User{}, err
	}

	return res.User, nil
}

func (c *Client) DeleteUser(ctx context.Context, password []byte, req DeleteUserReq) error {
	user, err := c.GetUser(ctx)
	if err != nil {
		return err
	}

	info, err := c.m.AuthInfo(ctx, AuthInfoReq{Username: user.Name})
	if err != nil {
		return err
	}

	srpAuth, err := srp.NewAuth(info.Version, user.Name, password, info.Salt, info.Modulus, info.ServerEphemeral)
	if err != nil {
		return err
	}

	proofs, err := srpAuth.GenerateProofs(2048)
	if err != nil {
		return err
	}

	return c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetBody(struct {
			DeleteUserReq
			AuthReq
		}{
			DeleteUserReq: req,
			AuthReq: AuthReq{
				ClientProof:     base64.StdEncoding.EncodeToString(proofs.ClientProof),
				ClientEphemeral: base64.StdEncoding.EncodeToString(proofs.ClientEphemeral),
				SRPSession:      info.SRPSession,
			},
		}).Delete("/core/v4/users/delete")
	})
}
