package s3

import (
	"errors"
	"testing"

	"github.com/golang/mock/gomock"
	"github.com/ncw/rclone/fs"
	"github.com/stretchr/testify/require"
)

const invalidHashMD5 = "invalid"
const validHashMD5Hex = "548db278711c85b61ade28625db3b0e2"
const validHashMD5Base64 = "VI2yeHEchbYa3ihiXbOw4g=="

func TestHash_ReturnsETag_WhenETagIsAValidHashMD5(t *testing.T) {
	hash, err := (&Object{etag: validHashMD5Hex}).Hash(fs.HashMD5)
	require.Equal(t, validHashMD5Hex, hash)
	require.Empty(t, err)
}

func TestHashMD5FromSrc_ReturnsError_WhenProxiedHashMD5IsNotSupported(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	hash, err := (&Object{}).hashMD5FromSrc(
		newMockObjectInfo(
			mockCtrl,
			validHashMD5Hex,
			errors.New("not supported"),
		),
	)
	require.Empty(t, hash)
	require.Error(t, err)
}

func TestHashMD5FromSrc_ReturnsError_WhenProxiedHashMD5IsInvalid(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	hash, err := (&Object{}).hashMD5FromSrc(
		newMockObjectInfo(
			mockCtrl,
			invalidHashMD5,
			nil,
		),
	)
	require.Empty(t, hash)
	require.Error(t, err)
}

func TestHashMD5FromSrc_ReturnsBase64HashMD5_WhenProxiedHashMD5IsValid(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	hash, err := (&Object{}).hashMD5FromSrc(
		newMockObjectInfo(
			mockCtrl,
			validHashMD5Hex,
			nil,
		),
	)
	require.Equal(t, validHashMD5Base64, hash)
	require.Empty(t, err)
}

func TestHashMD5FromMeta_ReturnsEmptyString_WhenMetaDoesNotContainKey(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	value := "value"
	hash, err := (&Object{}).hashMD5FromMeta(
		map[string]*string{
			"key": &value,
		},
	)
	require.Empty(t, hash)
	require.Empty(t, err)
}

func TestHashMD5FromMeta_ReturnsError_WhenMetaContainsInvalidHashMD5(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	value := invalidHashMD5
	hash, err := (&Object{}).hashMD5FromMeta(
		map[string]*string{
			metaMD5Hash: &value,
		},
	)
	require.Empty(t, hash)
	require.Error(t, err)
}

func TestHashMD5FromMeta_ReturnsHexHashMD5_WhenMetaContainsValidHashMD5(t *testing.T) {
	mockCtrl := gomock.NewController(t)
	defer mockCtrl.Finish()

	value := validHashMD5Base64
	hash, err := (&Object{}).hashMD5FromMeta(
		map[string]*string{
			metaMD5Hash: &value,
		},
	)
	require.Equal(t, validHashMD5Hex, hash)
	require.Empty(t, err)
}

func newMockObjectInfo(mockCtrl *gomock.Controller, hash string, err error) *fs.MockObjectInfo {
	mockObjectInfo := fs.NewMockObjectInfo(mockCtrl)
	mockObjectInfo.EXPECT().Hash(fs.HashMD5).DoAndReturn(func(t fs.HashType) (string, error) { return hash, err })

	return mockObjectInfo
}
