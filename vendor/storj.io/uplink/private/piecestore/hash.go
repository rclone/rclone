// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package piecestore

import (
	"context"

	"storj.io/common/pb"
)

type pieceHashAlgoKey struct{}

// WithPieceHashAlgo sets the used piece hash algorithm.
func WithPieceHashAlgo(ctx context.Context, hash pb.PieceHashAlgorithm) context.Context {
	return context.WithValue(ctx, pieceHashAlgoKey{}, hash)
}

// GetPieceHashAlgo returns with the piece hash algorithm which may be overridden.
func GetPieceHashAlgo(ctx context.Context) (algo pb.PieceHashAlgorithm) {
	override := ctx.Value(pieceHashAlgoKey{})
	if override != nil {
		return override.(pb.PieceHashAlgorithm)
	}
	return pb.PieceHashAlgorithm_SHA256
}
