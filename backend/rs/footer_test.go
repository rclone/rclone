package rs

import (
	"encoding/hex"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewRSFooterDefaults(t *testing.T) {
	ft := NewRSFooter(0, nil, nil, time.Unix(1700000000, 0), 3, 1, 0, 256*1024, 0, 0)
	require.Equal(t, CompressionNone, ft.Compression)
	require.Equal(t, uint32(0), ft.NumBlocks)
	require.Equal(t, AlgorithmRS, ft.Algorithm)
	require.Equal(t, emptyFileMD5, ft.MD5)
	require.Equal(t, emptyFileSHA256, ft.SHA256)
}

func TestFooterMarshalParseRoundTrip(t *testing.T) {
	var md5Sum [16]byte
	var sha256Sum [32]byte
	_, err := hex.Decode(md5Sum[:], []byte("d41d8cd98f00b204e9800998ecf8427e"))
	require.NoError(t, err)
	_, err = hex.Decode(sha256Sum[:], []byte("e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"))
	require.NoError(t, err)

	snappyTag := [4]byte{'s', 'z', ' ', ' '}
	cases := []Footer{
		*NewRSFooter(0, nil, nil, time.Unix(1700000000, 0), 3, 1, 0, 256*1024, 0, 0),
		*NewRSFooter(100, md5Sum[:], sha256Sum[:], time.Unix(1700001234, 0), 2, 2, 0, 64, 2, 0x12345678),
		*NewRSFooter(1<<20, md5Sum[:], sha256Sum[:], time.Unix(1700005678, 0), 4, 3, 3, 32*1024, 10, 0xabcdef01),
		*NewRSFooter(42, md5Sum[:], sha256Sum[:], time.Unix(1700009999, 0), 3, 1, 6, 128, 1, 0),
		{
			ContentLength: 512,
			MD5:           md5Sum,
			SHA256:        sha256Sum,
			Mtime:         1700011111,
			Compression:   snappyTag,
			NumBlocks:     7,
			Algorithm:     AlgorithmRS,
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
				copy(b[0:9], "BADMAGIC!")
				return b
			}(),
			want: "invalid magic",
		},
		{
			name: "unsupported version",
			buf: func() []byte {
				b := append([]byte{}, valid...)
				b[10] = byte(FooterVersion + 1)
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
