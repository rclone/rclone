package rs

import (
	"encoding/binary"
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewRSFooterDefaults(t *testing.T) {
	ft := NewRSFooter(0, nil, nil, time.Unix(1700000000, 0), 3, 1, 0, 256*1024, 0, 0)
	require.Equal(t, AlgorithmSYMM, ft.Algorithm)
	require.Equal(t, int64(1700000000)*1e9, ft.Mtime)
	require.Equal(t, emptyFileMD5, ft.MD5)
	require.Equal(t, emptyFileSHA256, ft.SHA256)
}

func TestFooterFieldOffsets(t *testing.T) {
	require.Equal(t, 0, 0%8)
	require.Equal(t, footerOffVersion, 8)
	require.Equal(t, footerOffAlgorithm, 12)
	require.Equal(t, footerOffContentLength, 16)
	require.Equal(t, footerOffContentLength%8, 0)
	require.Equal(t, footerOffMD5, 24)
	require.Equal(t, footerOffMD5%8, 0)
	require.Equal(t, footerOffSHA256, 40)
	require.Equal(t, footerOffSHA256%8, 0)
	require.Equal(t, footerOffMtime, 72)
	require.Equal(t, footerOffMtime%8, 0)
	require.Equal(t, footerOffStripeSize, 80)
	require.Equal(t, footerOffStripeSize%4, 0)
	require.Equal(t, footerOffNumStripes, 84)
	require.Equal(t, footerOffPayloadCRC32C, 88)
	require.Equal(t, FooterSize%8, 0)
}

func TestFooterMarshalParseRoundTrip(t *testing.T) {
	var md5Sum [16]byte
	var sha256Sum [32]byte
	_, err := hex.Decode(md5Sum[:], []byte("d41d8cd98f00b204e9800998ecf8427e"))
	require.NoError(t, err)
	_, err = hex.Decode(sha256Sum[:], []byte("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
	require.NoError(t, err)

	subSec := time.Unix(1, 123456789)
	cases := []Footer{
		*NewRSFooter(0, nil, nil, time.Unix(1700000000, 0), 3, 1, 0, 256*1024, 0, 0),
		*NewRSFooter(100, md5Sum[:], sha256Sum[:], time.Unix(1700001234, 0), 2, 2, 0, 64, 2, 0x12345678),
		*NewRSFooter(1<<20, md5Sum[:], sha256Sum[:], subSec, 4, 3, 3, 32*1024, 10, 0xabcdef01),
		*NewRSFooter(42, md5Sum[:], sha256Sum[:], time.Unix(1700009999, 0), 3, 1, 6, 128, 1, 0),
		{
			ContentLength: 512,
			MD5:           md5Sum,
			SHA256:        sha256Sum,
			Mtime:         subSec.UnixNano(),
			Algorithm:     AlgorithmSYMM,
			DataShards:    3,
			ParityShards:  1,
			CurrentShard:  2,
			StripeSize:    65536,
			PayloadCRC32C: 0xdeadbeef,
			NumStripes:    3,
		},
	}

	for i, want := range cases {
		raw, err := want.MarshalBinary()
		require.NoError(t, err, "case %d marshal", i)
		require.Len(t, raw, FooterSize)
		require.Equal(t, FooterMagic[:], raw[0:8])
		require.Equal(t, uint32(FooterVersion), binary.LittleEndian.Uint32(raw[footerOffVersion:]))
		require.Equal(t, want.Mtime, int64(binary.LittleEndian.Uint64(raw[footerOffMtime:])), "case %d mtime bytes", i)

		got, err := ParseFooter(raw)
		require.NoError(t, err, "case %d parse", i)
		require.Equal(t, &want, got, "case %d round-trip", i)
	}
}

func TestParseFooterErrors(t *testing.T) {
	valid, err := NewRSFooter(10, nil, nil, time.Unix(0, 0), 2, 1, 0, 64, 1, 0).MarshalBinary()
	require.NoError(t, err)

	cases := []struct {
		name string
		buf  []byte
		want string
	}{
		{
			name: "short buffer",
			buf:  valid[:FooterSize-1],
			want: "buffer length must be",
		},
		{
			name: "long buffer",
			buf:  append(append([]byte{}, valid...), 0),
			want: "buffer length must be",
		},
		{
			name: "bad magic",
			buf: func() []byte {
				b := append([]byte{}, valid...)
				copy(b[0:8], "BADMAGIC")
				return b
			}(),
			want: "invalid magic",
		},
		{
			name: "unsupported version",
			buf: func() []byte {
				b := append([]byte{}, valid...)
				binary.LittleEndian.PutUint32(b[footerOffVersion:], FooterVersion+1)
				return b
			}(),
			want: "unsupported version",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFooter(tc.buf)
			require.Error(t, err)
			require.True(t, strings.Contains(err.Error(), tc.want), "got %v", err)
		})
	}
}
