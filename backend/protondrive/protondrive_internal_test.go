package protondrive

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestShouldRetry(t *testing.T) {
	ctx := context.Background()
	cancelledCtx, cancel := context.WithCancel(ctx)
	cancel()

	for _, tc := range []struct {
		name      string
		ctx       context.Context
		err       error
		wantRetry bool
	}{
		{"nil error", ctx, nil, false},
		{"cancelled context", cancelledCtx, errors.New("some error"), false},
		{"storage block error Code=200501", ctx, errors.New("422 POST: Operation failed (Code=200501)"), true},
		{"rate limit Status=429", ctx, errors.New("Status=429 Too Many Requests"), true},
		{"server error Status=500", ctx, errors.New("Status=500 Internal Server Error"), true},
		{"server error Status=502", ctx, errors.New("Status=502 Bad Gateway"), true},
		{"server error Status=503", ctx, errors.New("Status=503 Service Unavailable"), true},
		{"server error Status=504", ctx, errors.New("Status=504 Gateway Timeout"), true},
		{"client error Status=400", ctx, errors.New("Status=400 Bad Request"), false},
		{"client error Status=404", ctx, errors.New("Status=404 Not Found"), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gotRetry, _ := shouldRetry(tc.ctx, tc.err)
			assert.Equal(t, tc.wantRetry, gotRetry)
		})
	}
}
