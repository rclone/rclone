package odrvcookie

import (
	"time"

	"github.com/ncw/rclone/lib/rest"
)

// CookieRenew holds information for the renew
type CookieRenew struct {
	srv     *rest.Client
	timer   *time.Ticker
	renewFn func()
}

// NewRenew returns and starts a CookieRenew
func NewRenew(interval time.Duration, renewFn func()) *CookieRenew {
	renew := CookieRenew{
		timer:   time.NewTicker(interval),
		renewFn: renewFn,
	}
	go renew.Renew()
	return &renew
}

// Renew calls the renewFn for every tick
func (c *CookieRenew) Renew() {
	for {
		<-c.timer.C // wait for tick
		c.renewFn()
	}
}
