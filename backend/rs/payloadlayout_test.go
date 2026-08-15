package rs

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/object"
	"github.com/stretchr/testify/require"
)

func TestPayloadLayoutExamples(t *testing.T) {
	t.Run("exampleA", func(t *testing.T) {
		const k, m, S = 2, 2, 32
		cl := int64(1)
		require.Equal(t, 1, NumStripesForContent(cl, k, S))
		require.Equal(t, int64(1), DataShardPayloadLen(cl, 0, k, S))
		require.Equal(t, int64(0), DataShardPayloadLen(cl, 1, k, S))
		require.Equal(t, int64(32), ParityShardPayloadLen(cl, k, S))
		require.Equal(t, int64(1+FooterSize), ExpectedParticleSize(cl, 0, k, m, S, true))
	})

	t.Run("exampleB", func(t *testing.T) {
		const k, m, S = 2, 2, 32
		cl := int64(100)
		require.Equal(t, 2, NumStripesForContent(cl, k, S))
		require.Equal(t, int64(64), DataShardPayloadLen(cl, 0, k, S))
		require.Equal(t, int64(36), DataShardPayloadLen(cl, 1, k, S))
		require.Equal(t, int64(64), ParityShardPayloadLen(cl, k, S))
		require.Equal(t, int64(64+FooterSize), ExpectedParticleSize(cl, 0, k, m, S, true))
		require.Equal(t, int64(36+FooterSize), ExpectedParticleSize(cl, 1, k, m, S, true))
		require.Equal(t, int64(64+FooterSize), ExpectedParticleSize(cl, 2, k, m, S, true))
	})

	t.Run("exampleC", func(t *testing.T) {
		const k, m, S = 2, 2, 32
		cl := int64(0)
		require.Equal(t, 0, NumStripesForContent(cl, k, S))
		require.Equal(t, int64(FooterSize), ExpectedParticleSize(cl, 0, k, m, S, true))
	})

	t.Run("exampleD", func(t *testing.T) {
		const k, m, S = 2, 1, 16
		cl := int64(18)
		require.Equal(t, 1, NumStripesForContent(cl, k, S))
		require.Equal(t, int64(16), DataShardPayloadLen(cl, 0, k, S))
		require.Equal(t, 2, DataShardFragLen(1, k, S, 18))
		require.Equal(t, int64(16), ParityShardPayloadLen(cl, k, S))
	})

	t.Run("validateShardParticleFile", func(t *testing.T) {
		const k, m, S = 2, 2, 32
		cl := int64(100)
		require.NoError(t, ValidateShardParticleFile(64+FooterSize, cl, 0, k, m, S))
		require.NoError(t, ValidateShardParticleFile(36+FooterSize, cl, 1, k, m, S))
		require.Error(t, ValidateShardParticleFile(64+FooterSize, cl, 1, k, m, S))
	})

	t.Run("contentLengthFromDataShardPayloads", func(t *testing.T) {
		const k = 2
		sum, ok := ContentLengthFromDataShardPayloads([]int64{64 + FooterSize, 36 + FooterSize}, k)
		require.True(t, ok)
		require.Equal(t, int64(100), sum)
	})
}

func TestEncodeValidateExtractRoundTrip(t *testing.T) {
	ctx := context.Background()
	const k, m, S = 2, 2, 32
	data := bytes.Repeat([]byte("xy"), 50)
	src := object.NewStaticObjectInfo("t.bin", time.Unix(1700000000, 0), int64(len(data)), true, nil, nil)
	writers := make([]*bytes.Buffer, k+m)
	ios := make([]io.Writer, k+m)
	for i := range writers {
		writers[i] = &bytes.Buffer{}
		ios[i] = writers[i]
	}
	res, err := BuildRSShardsToWriters(ctx, bytes.NewReader(data), src, k, m, S, ios, true)
	require.NoError(t, err)
	for i := range writers {
		particle := writers[i].Bytes()
		require.NoError(t, ValidateShardParticleFile(int64(len(particle)), res.ContentLength, i, k, m, S))
		payload, ft, err := ExtractParticlePayload(particle, i)
		require.NoError(t, err, "shard %d", i)
		require.Equal(t, res.ContentLength, ft.ContentLength)
		if i < k {
			_ = payload
		}
	}
	decoded, err := ReconstructDataFromShards(shardPayloadsFromWriters(writers), k, m, res.ContentLength, res.StripeSize, res.NumStripes)
	require.NoError(t, err)
	require.Equal(t, data, decoded)
}

func shardPayloadsFromWriters(writers []*bytes.Buffer) [][]byte {
	out := make([][]byte, len(writers))
	for i, w := range writers {
		particle := w.Bytes()
		payload, _, err := ExtractParticlePayload(particle, i)
		if err != nil {
			panic(fmt.Sprintf("shard %d: %v", i, err))
		}
		out[i] = payload
	}
	return out
}
