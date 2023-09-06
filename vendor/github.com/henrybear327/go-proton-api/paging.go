package proton

import (
	"context"
	"runtime"

	"github.com/ProtonMail/gluon/async"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/parallel"
	"github.com/bradenaw/juniper/stream"
)

const maxPageSize = 150

func fetchPaged[T any](
	ctx context.Context,
	total, pageSize int, c *Client,
	fn func(ctx context.Context, page, pageSize int) ([]T, error),
) ([]T, error) {
	return stream.Collect(ctx, stream.Flatten(parallel.MapStream(
		ctx,
		stream.FromIterator(iterator.Counter(total/pageSize+1)),
		runtime.NumCPU(),
		runtime.NumCPU(),
		func(ctx context.Context, page int) (stream.Stream[T], error) {
			defer async.HandlePanic(c.m.panicHandler)

			values, err := fn(ctx, page, pageSize)
			if err != nil {
				return nil, err
			}

			return stream.FromIterator(iterator.Slice(values)), nil
		},
	)))
}
