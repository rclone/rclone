package smb2

import (
	"context"
	"sync"
)

type account struct {
	m        sync.Mutex
	balance  chan struct{}
	_opening uint16
}

func openAccount(maxCreditBalance uint16) *account {
	balance := make(chan struct{}, maxCreditBalance)

	balance <- struct{}{} // initial balance

	return &account{
		balance: balance,
	}
}

func (a *account) initRequest() uint16 {
	return uint16(cap(a.balance) - len(a.balance))
}

func (a *account) loan(creditCharge uint16, ctx context.Context) (uint16, bool, error) {
	select {
	case <-a.balance:
	case <-ctx.Done():
		return 0, false, &ContextError{Err: ctx.Err()}
	}

	for i := uint16(1); i < creditCharge; i++ {
		select {
		case <-a.balance:
		default:
			return i, false, nil
		}
	}

	return creditCharge, true, nil
}

func (a *account) opening() uint16 {
	a.m.Lock()

	ret := a._opening
	a._opening = 0

	a.m.Unlock()

	return ret
}

func (a *account) charge(granted, requested uint16) {
	if granted == 0 && requested == 0 {
		return
	}

	a.m.Lock()

	if granted < requested {
		a._opening += requested - granted
	}

	a.m.Unlock()

	for i := uint16(0); i < granted; i++ {
		select {
		case a.balance <- struct{}{}:
		default:
			return
		}
	}
}
