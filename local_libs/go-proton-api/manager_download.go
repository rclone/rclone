package proton

import (
	"context"
	"io"

	"github.com/ProtonMail/gopenpgp/v2/crypto"
)

func (m *Manager) DownloadAndVerify(ctx context.Context, kr *crypto.KeyRing, url, sig string) ([]byte, error) {
	fb, err := m.fetchFile(ctx, url)
	if err != nil {
		return nil, err
	}

	sb, err := m.fetchFile(ctx, sig)
	if err != nil {
		return nil, err
	}

	if err := kr.VerifyDetached(
		crypto.NewPlainMessage(fb),
		crypto.NewPGPSignature(sb),
		crypto.GetUnixTime(),
	); err != nil {
		return nil, err
	}

	return fb, nil
}

func (m *Manager) fetchFile(ctx context.Context, url string) ([]byte, error) {
	res, err := m.r(ctx).SetDoNotParseResponse(true).Get(url)
	if err != nil {
		return nil, err
	}

	b, err := io.ReadAll(res.RawBody())
	if err != nil {
		return nil, err
	}

	if err := res.RawBody().Close(); err != nil {
		return nil, err
	}

	return b, nil
}
