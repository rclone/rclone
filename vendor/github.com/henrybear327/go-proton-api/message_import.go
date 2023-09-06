package proton

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/ProtonMail/gluon/async"
	"github.com/ProtonMail/gopenpgp/v2/crypto"
	"github.com/bradenaw/juniper/iterator"
	"github.com/bradenaw/juniper/parallel"
	"github.com/bradenaw/juniper/stream"
	"github.com/bradenaw/juniper/xslices"
	"github.com/go-resty/resty/v2"
)

const (
	// maxImportCount is the maximum number of messages that can be imported in a single request.
	maxImportCount = 10

	// maxImportSize is the maximum total request size permitted for a single import request.
	maxImportSize = 30 * 1024 * 1024
)

var ErrImportEncrypt = errors.New("failed to encrypt message")
var ErrImportSizeExceeded = errors.New("message exceeds maximum import size of 30MB")

func (c *Client) ImportMessages(ctx context.Context, addrKR *crypto.KeyRing, workers, buffer int, req ...ImportReq) (stream.Stream[ImportRes], error) {
	// Encrypt each message.
	for idx := range req {
		enc, err := EncryptRFC822(addrKR, req[idx].Message)
		if err != nil {
			return nil, fmt.Errorf("%w %v: %v", ErrImportEncrypt, idx, err)
		}

		req[idx].Message = enc
	}

	// If any of the messages exceed the maximum import size, return an error.
	if xslices.Any(req, func(req ImportReq) bool { return len(req.Message) > maxImportSize }) {
		return nil, ErrImportSizeExceeded
	}

	return stream.Flatten(parallel.MapStream(
		ctx,
		stream.FromIterator(iterator.Slice(chunkSized(req, maxImportCount, maxImportSize, func(req ImportReq) int {
			return len(req.Message)
		}))),
		workers,
		buffer,
		func(ctx context.Context, req []ImportReq) (stream.Stream[ImportRes], error) {
			defer async.HandlePanic(c.m.panicHandler)

			res, err := c.importMessages(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to import messages: %w", err)
			}

			for _, res := range res {
				if res.Code != SuccessCode {
					return nil, fmt.Errorf("failed to import message: %w", res.APIError)
				}
			}

			return stream.FromIterator(iterator.Slice(res)), nil
		},
	)), nil
}

func (c *Client) importMessages(ctx context.Context, req []ImportReq) ([]ImportRes, error) {
	names := iterator.Collect(iterator.Map(iterator.Counter(len(req)), func(i int) string {
		return strconv.Itoa(i)
	}))

	var named []namedImportReq

	for idx, name := range names {
		named = append(named, namedImportReq{
			ImportReq: req[idx],
			Name:      name,
		})
	}

	type namedImportRes struct {
		Name     string
		Response ImportRes
	}

	var res struct {
		Responses []namedImportRes
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		fields, err := buildImportReqFields(named)
		if err != nil {
			return nil, err
		}

		return r.SetMultipartFields(fields...).SetResult(&res).Post("/mail/v4/messages/import")
	}); err != nil {
		return nil, err
	}

	namedRes := make(map[string]ImportRes, len(res.Responses))

	for _, res := range res.Responses {
		namedRes[res.Name] = res.Response
	}

	return xslices.Map(names, func(name string) ImportRes {
		return namedRes[name]
	}), nil
}

// chunkSized splits a slice into chunks of maximum size and length.
// It is assumed that the size of each element is less than the maximum size.
func chunkSized[T any](vals []T, maxLen, maxSize int, getSize func(T) int) [][]T {
	var chunks [][]T

	for len(vals) > 0 {
		var (
			curChunk []T
			curSize  int
		)

		for len(vals) > 0 && len(curChunk) < maxLen && curSize < maxSize {
			val, size := vals[0], getSize(vals[0])

			if curSize+size > maxSize {
				break
			}

			curChunk = append(curChunk, val)
			curSize += size
			vals = vals[1:]
		}

		chunks = append(chunks, curChunk)
	}

	return chunks
}
