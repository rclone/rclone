package proton

import (
	"math/rand"
	"time"

	"github.com/ProtonMail/gluon/async"
)

type Ticker struct {
	C chan time.Time

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewTicker returns a new ticker that ticks at a random time between period and period+jitter.
// It can be stopped by closing calling Stop().
func NewTicker(period, jitter time.Duration, panicHandler async.PanicHandler) *Ticker {
	t := &Ticker{
		C:      make(chan time.Time),
		stopCh: make(chan struct{}),
		doneCh: make(chan struct{}),
	}

	go func() {
		defer async.HandlePanic(panicHandler)

		defer close(t.doneCh)

		for {
			select {
			case <-t.stopCh:
				return

			case <-time.After(withJitter(period, jitter)):
				select {
				case <-t.stopCh:
					return

				case t.C <- time.Now():
					// ...
				}
			}
		}
	}()

	return t
}

func (t *Ticker) Stop() {
	close(t.stopCh)
	<-t.doneCh
}

func withJitter(period, jitter time.Duration) time.Duration {
	if jitter == 0 {
		return period
	}

	return period + time.Duration(rand.Int63n(int64(jitter)))
}
