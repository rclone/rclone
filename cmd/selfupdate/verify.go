package selfupdate

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	"github.com/rclone/rclone/fs"
	"golang.org/x/crypto/openpgp"
	"golang.org/x/crypto/openpgp/clearsign"
)

var ncwPublicKeyPGP = `-----BEGIN PGP PUBLIC KEY BLOCK-----

mQGiBDuy3V0RBADVQOAF5aFiCxD3t2h6iAF2WMiaMlgZ6kX2i/u7addNkzX71VU9
7NpI0SnsP5YWt+gEedST6OmFbtLfZWCR4KWn5XnNdjCMNhxaH6WccVqNm4ALPIqT
59uVjkgf8RISmmoNJ1d+2wMWjQTUfwOEmoIgH6n+2MYNUKuctBrwAACflwCg1I1Q
O/prv/5hczdpQCs+fL87DxsD/Rt7pIXvsIOZyQWbIhSvNpGalJuMkW5Jx92UjsE9
1Ipo3Xr6SGRPgW9+NxAZAsiZfCX/19knAyNrN9blwL0rcPDnkhdGwK69kfjF+wq+
QbogRGodbKhqY4v+cMNkKiemBuTQiWPkpKjifwNsD1fNjNKfDP3pJ64Yz7a4fuzV
X1YwBACpKVuEen34lmcX6ziY4jq8rKibKBs4JjQCRO24kYoHDULVe+RS9krQWY5b
e0foDhru4dsKccefK099G+WEzKVCKxupstWkTT/iJwajR8mIqd4AhD0wO9W3MCfV
Ov8ykMDZ7qBWk1DHc87Ep3W1o8t8wq74ifV+HjhhWg8QAylXg7QlTmljayBDcmFp
Zy1Xb29kIDxuaWNrQGNyYWlnLXdvb2QuY29tPohxBBMRCAAxBQsHCgMEAxUDAgMW
AgECF4AWIQT79zfs6firGGBL0qyTk14C/ztU+gUCXjg2UgIZAQAKCRCTk14C/ztU
+lmmAJ4jH5FyULzStjisuTvHLTVz6G44eQCfaR5QGZFPseenE5ic2WeQcBcmtoG5
Ag0EO7LdgRAIAI6QdFBg3/xa1gFKPYy1ihV9eSdGqwWZGJvokWsfCvHy5180tj/v
UNOLAJrdqglMSvevNTXe8bT65D6423AAsLhch9wq/aNqrHolTYABzxRigjcS1//T
yln5naGUzlVQXDVfrDk3Md/NrkdOFj7r/YyMF0+iWwpFz2qAjL95i5wfVZ1kWGrT
2AmivE1wD1sWT/Ja3FDI0NRkU0Nbz/a0TKe4ml8iLVtZXpTRbxxCCPdkHXXgSyu1
eZ4NrF/wTJuvwGn12TJ1EF95aVkHxAUw0+KmLGdcyBG+IKuHamrsjWIAXGXV///K
AxPgUthccQ03HMjltFsrdmen5Q034YM3eOsAAwUH/jAKiIAA8LpZmZPnt9GZ4+Ol
Zp22VAfyfDOFl4Ol+cWjkLAgjAFsm5gnOKcRSE/9XPxnQqkhw7+ZygYuUMgTDJ99
/5IM1UQL3ooS+oFrDaE99S8bLeOe17skcdXcA/K83VqD9m93rQRnbtD+75zqKkZn
9WNFyKCXg5P6PFPdNYRtlQKOcwFR9mHRLUmapQSAM8Y2pCgALZ7GViKQca8/TT1T
gZk9fJMZYGez+IlOPxTJxjn80+vywk4/wdIWSiQj+8u5RzT9sjmm77wbMVNGRqYd
W/EemW9Zz9vi0CIvJGgbPMqcuxw8e/5lnuQ6Mi3uDR0P2RNIAhFrdZpVSME8xQaI
RgQYEQIABgUCO7LdgQAKCRCTk14C/ztU+mLBAKC2cdFy7eLaQAvyzcE2VK6HVIjn
JACguA00bxLQuJ4+RCJrLFZP8ZlN2sc=
=TtR5
-----END PGP PUBLIC KEY BLOCK-----`

func verifyHashsum(ctx context.Context, siteURL, version, archive string, hash []byte) error {
	sumsURL := fmt.Sprintf("%s/%s/SHA256SUMS", siteURL, version)
	sumsBuf, err := downloadFile(ctx, sumsURL)
	if err != nil {
		return err
	}
	fs.Debugf(nil, "downloaded hashsum list: %s", sumsURL)

	keyRing, err := openpgp.ReadArmoredKeyRing(strings.NewReader(ncwPublicKeyPGP))
	if err != nil {
		return errors.New("unsupported signing key")
	}
	block, rest := clearsign.Decode(sumsBuf)
	// block.Bytes = block.Bytes[1:] // uncomment to test invalid signature
	_, err = openpgp.CheckDetachedSignature(keyRing, bytes.NewReader(block.Bytes), block.ArmoredSignature.Body)
	if err != nil || len(rest) > 0 {
		return errors.New("invalid hashsum signature")
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
