package proton

import (
	"context"
	"runtime"
	"time"

	"github.com/bradenaw/juniper/parallel"
	"github.com/go-resty/resty/v2"
)

func (c *Client) UndoActions(ctx context.Context, tokens ...UndoToken) ([]UndoRes, error) {
	return parallel.MapContext(ctx, runtime.NumCPU(), tokens, func(ctx context.Context, token UndoToken) (UndoRes, error) {
		if time.Unix(token.ValidUntil, 0).Before(time.Now()) {
			return UndoRes{}, ErrUndoTokenExpired
		}

		var res UndoRes

		if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
			return r.SetBody(token).SetResult(&res).Post("/mail/v4/undoactions")
		}); err != nil {
			return UndoRes{}, err
		}

		return res, nil
	})
}
