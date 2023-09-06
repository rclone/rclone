package proton

import (
	"context"
)

func (m *Manager) GetCaptcha(ctx context.Context, token string) ([]byte, error) {
	res, err := m.r(ctx).SetQueryParam("Token", token).SetQueryParam("ForceWebMessaging", "1").Get("/core/v4/captcha")
	if err != nil {
		return nil, err
	}

	return res.Body(), nil
}

func (m *Manager) SendVerificationCode(ctx context.Context, req SendVerificationCodeReq) error {
	if _, err := m.r(ctx).SetBody(req).Post("/core/v4/users/code"); err != nil {
		return err
	}

	return nil
}

func (m *Manager) CreateUser(ctx context.Context, req CreateUserReq) (User, error) {
	var res struct {
		User User
	}

	if _, err := m.r(ctx).SetBody(req).SetResult(&res).Post("/core/v4/users"); err != nil {
		return User{}, err
	}

	return res.User, nil
}

func (m *Manager) GetUsernameAvailable(ctx context.Context, username string) error {
	if _, err := m.r(ctx).SetQueryParam("Name", username).Get("/core/v4/users/available"); err != nil {
		return err
	}

	return nil
}
