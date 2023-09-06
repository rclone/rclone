package utils

import (
	"context"
	"math/rand"
	"time"
)

type JitteredTicker struct {
	C        chan struct{}
	interval time.Duration
}

func NewJitteredTicker(interval time.Duration) *JitteredTicker {
	return &JitteredTicker{
		C:        make(chan struct{}, 1),
		interval: interval,
	}
}

func (t *JitteredTicker) Run(ctx context.Context) {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	timer := time.NewTimer(Jitter(r, t.interval))
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			select {
			case t.C <- struct{}{}:
			case <-ctx.Done():
				return
			}
			timer.Reset(Jitter(r, t.interval))
		}
	}
}

func Jitter(r *rand.Rand, t time.Duration) time.Duration {
	nanos := r.NormFloat64()*float64(t/4) + float64(t)
	if nanos <= 0 {
		nanos = 1
	}
	return time.Duration(nanos)
}
