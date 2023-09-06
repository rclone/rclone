package proton

import "context"

func (m *Manager) GetDomains(ctx context.Context) ([]string, error) {
	var res struct {
		Domains []string
	}

	if _, err := m.r(ctx).SetResult(&res).Get("/core/v4/domains/available"); err != nil {
		return nil, err
	}

	return res.Domains, nil
}
