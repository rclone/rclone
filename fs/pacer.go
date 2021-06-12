// Pacer with logging and calculator

package fs

import (
	"context"
	"time"

	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/pacer"
)

// Pacer is a simple wrapper around a pacer.Pacer with logging.
type Pacer struct {
	*pacer.Pacer
}

type logCalculator struct {
	pacer.Calculator
}

// NewPacer creates a Pacer for the given Fs and Calculator.
func NewPacer(ctx context.Context, c pacer.Calculator) *Pacer {
	ci := GetConfig(ctx)
	retries := ci.LowLevelRetries
	if retries <= 0 {
		retries = 1
	}
	p := &Pacer{
		Pacer: pacer.New(
			pacer.InvokerOption(pacerInvoker),
			pacer.MaxConnectionsOption(ci.Checkers+ci.Transfers),
			pacer.RetriesOption(retries),
			pacer.CalculatorOption(c),
		),
	}
	p.SetCalculator(c)
	return p
}

func (d *logCalculator) Calculate(state pacer.State) time.Duration {
	oldSleepTime := state.SleepTime
	newSleepTime := d.Calculator.Calculate(state)
	if state.ConsecutiveRetries > 0 {
		if newSleepTime != oldSleepTime {
			Debugf("pacer", "Rate limited, increasing sleep to %v", newSleepTime)
		}
	} else {
		if newSleepTime != oldSleepTime {
			Debugf("pacer", "Reducing sleep to %v", newSleepTime)
		}
	}
	return newSleepTime
}

// SetCalculator sets the pacing algorithm. Don't modify the Calculator object
// afterwards, use the ModifyCalculator method when needed.
//
// It will choose the default algorithm if nil is passed in.
func (p *Pacer) SetCalculator(c pacer.Calculator) {
	switch c.(type) {
	case *logCalculator:
		Logf("pacer", "Invalid Calculator in fs.Pacer.SetCalculator")
	case nil:
		c = &logCalculator{pacer.NewDefault()}
	default:
		c = &logCalculator{c}
	}

	p.Pacer.SetCalculator(c)
}

// ModifyCalculator calls the given function with the currently configured
// Calculator and the Pacer lock held.
func (p *Pacer) ModifyCalculator(f func(pacer.Calculator)) {
	p.ModifyCalculator(func(c pacer.Calculator) {
		switch _c := c.(type) {
		case *logCalculator:
			f(_c.Calculator)
		default:
			Logf("pacer", "Invalid Calculator in fs.Pacer: %t", c)
			f(c)
		}
	})
}

func pacerInvoker(try, retries int, f pacer.Paced) (retry bool, err error) {
	retry, err = f()
	if retry {
		Debugf("pacer", "low level retry %d/%d (error %v)", try, retries, err)
		err = fserrors.RetryError(err)
	}
	return
}
