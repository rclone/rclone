package proton

import "context"

func (m *Manager) Ping(ctx context.Context) error {
	if res, err := m.r(ctx).Get("/tests/ping"); err != nil {
		if res.RawResponse != nil {
			return nil
		}

		return err
	}

	return nil
}
