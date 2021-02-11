package filename

import "errors"

const scsuNotEnabled = "scsu encoding not enabled in this build due to old go version"

// Functions wrap scsu package, since it doesn't build on old Go versions.
// Remove once v1.13 is minimum supported version.

var scsuDecode = func(b []byte) (string, error) {
	return "", errors.New(scsuNotEnabled)
}

var scsuEncodeStrict = func(src string, dst []byte) ([]byte, error) {
	return nil, errors.New(scsuNotEnabled)
}
