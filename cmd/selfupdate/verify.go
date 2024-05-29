//go:build !noselfupdate

package selfupdate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/clearsign"
	"github.com/rclone/rclone/fs"
)

var ncwPublicKeyPGP = `-----BEGIN PGP PUBLIC KEY BLOCK-----

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
EtC4nj5EImssVk/xmU3axw==
=VUqh
-----END PGP PUBLIC KEY BLOCK-----
`

func verifyHashsum(ctx context.Context, siteURL, version, archive string, hash []byte) error {
	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS", siteURL, version)
	sumsBuf, err := downloadFile(ctx, sumsURL)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "downloaded hashsum list: %s", sumsURL)
	return verifyHashsumDownloaded(ctx, sumsBuf, archive, hash)
}

func verifyHashsumDownloaded(ctx context.Context, sumsBuf []byte, archive string, hash []byte) error {
	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(ncwPublicKeyPGP))
	if err != nil {
		return fmt.Errorf("unsupported signing key: %w", err)
	}

	block, rest := clearsign.Decode(sumsBuf)
	if block == nil {
		return errors.New("invalid hashsum signature: couldn't find detached signature")
	}
	if len(rest) > 0 {
		return fmt.Errorf("invalid hashsum signature: %d bytes of unsigned data", len(rest))
	}

	_, err = openpgp.CheckDetachedSignature(keyRing, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body, nil)
	if err != nil {
		return fmt.Errorf("invalid hashsum signature: %w", err)
	}

	wantHash, err := findFileHash(sumsBuf, archive)
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, wantHash) {
		return fmt.Errorf("archive hash mismatch: want %02x vs got %02x", wantHash, hash)
	}
	return nil
}
