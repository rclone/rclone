package rs

import (
	"context"
	"fmt"
	"time"

	"golang.org/x/sync/errgroup"
)

const healthCheckTimeout = 15 * time.Second

type backendHealth struct {
	Index     int
	Name      string
	Available bool
	Err       error
}

func (f *Fs) collectBackendHealth(ctx context.Context) []backendHealth {
	out := make([]backendHealth, len(f.backends))
	g, gctx := errgroup.WithContext(ctx)
	for i, b := range f.backends {
		i := i
		b := b
		g.Go(func() error {
			_, err := b.List(gctx, "")
			out[i] = backendHealth{
				Index:     i,
				Name:      b.Name(),
				Available: err == nil,
				Err:       err,
			}
			return nil
		})
	}
	_ = g.Wait()
	return out
}

func (f *Fs) statusText(ctx context.Context) string {
	checkCtx, cancel := context.WithTimeout(ctx, healthCheckTimeout)
	defer cancel()
	health := f.collectBackendHealth(checkCtx)
	available := 0
	for _, h := range health {
		if h.Available {
			available++
		}
	}

	status := "HEALTHY"
	if available < f.writeQuorum() {
		status = "DEGRADED (WRITE BLOCKED)"
	}
	out := "RS Backend Health Status\n"
	out += "========================================\n"
	out += fmt.Sprintf("Data shards (k):   %d\n", f.opt.DataShards)
	out += fmt.Sprintf("Parity shards (m): %d\n", f.opt.ParityShards)
	out += fmt.Sprintf("Write quorum:      %d\n", f.writeQuorum())
	out += fmt.Sprintf("Available remotes: %d/%d\n", available, len(f.backends))
	out += fmt.Sprintf("Overall status:    %s\n\n", status)
	out += "Per-remote:\n"
	for _, h := range health {
		line := fmt.Sprintf("  - shard %d (%s): ", h.Index, h.Name)
		if h.Available {
			line += "OK\n"
		} else {
			line += fmt.Sprintf("UNAVAILABLE (%v)\n", h.Err)
		}
		out += line
	}
	return out
}
