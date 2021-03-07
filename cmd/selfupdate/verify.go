package selfupdate

import (
	"bytes"
	"context"
	"fmt"

	"github.com/rclone/rclone/fs"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/clearsign"
)

var ncwPublicPgpKey = []byte(`
-----BEGIN PGP PUBLIC KEY BLOCK-----

FIXME put a valid public release key here!
-----END PGP PUBLIC KEY BLOCK-----
`)

func verifyHashsum(ctx context.Context, siteURL, version, archive string, hash []byte) error {
	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS", siteURL, version)
	sumsBuf, err := downloadFile(ctx, sumsURL)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "downloaded hashsum list: %s", sumsURL)

	keyRing, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(ncwPublicPgpKey))
	if err != nil {
		fs.Infof(nil, "failed to read public key: %v (is it a PGP-2 key?)", err) // FIXME should fail
	}
	block, _ := clearsign.Decode(sumsBuf)
	msgBody := bytes.NewReader(block.Plaintext)
	sigBody := block.ArmoredSignature.Body
	_, err = openpgp.CheckArmoredDetachedSignature(keyRing, msgBody, sigBody)
	if err != nil {
		fs.Infof(nil, "PGP signature verification failed: %v", err) // FIXME should fail
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
