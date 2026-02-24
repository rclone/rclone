package proton

import (
	"context"
	"errors"
	"net"
	"sync"

	"github.com/ProtonMail/gluon/async"
	"github.com/go-resty/resty/v2"
)

type Manager struct {
	rc *resty.Client

	status     Status
	observers  []StatusObserver
	statusLock sync.Mutex

	errHandlers map[Code][]Handler

	verifyProofs bool

	panicHandler async.PanicHandler
}

func New(opts ...Option) *Manager {
	builder := newManagerBuilder()

	for _, opt := range opts {
		opt.config(builder)
	}

	return builder.build()
}

func (m *Manager) AddStatusObserver(observer StatusObserver) {
	m.statusLock.Lock()
	defer m.statusLock.Unlock()

	m.observers = append(m.observers, observer)
}

func (m *Manager) AddPreRequestHook(hook resty.RequestMiddleware) {
	m.rc.OnBeforeRequest(hook)
}

func (m *Manager) AddPostRequestHook(hook resty.ResponseMiddleware) {
	m.rc.OnAfterResponse(hook)
}

func (m *Manager) AddErrorHandler(code Code, handler Handler) {
	m.errHandlers[code] = append(m.errHandlers[code], handler)
}

func (m *Manager) Close() {
	m.rc.GetClient().CloseIdleConnections()
}

func (m *Manager) r(ctx context.Context) *resty.Request {
	return m.rc.R().SetContext(ctx)
}

func (m *Manager) handleError(req *resty.Request, err error) {
	resErr, ok := err.(*resty.ResponseError)
	if !ok {
		return
	}

	apiErr, ok := resErr.Response.Error().(*APIError)
	if !ok {
		return
	}

	for _, handler := range m.errHandlers[apiErr.Code] {
		handler()
	}
}

func (m *Manager) checkConnUp(_ *resty.Client, res *resty.Response) error {
	m.onConnUp()

	return nil
}

func (m *Manager) checkConnDown(req *resty.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled):
		return
	}

	if res, ok := err.(*resty.ResponseError); ok {
		if res.Response.RawResponse == nil {
			m.onConnDown()
		} else if netErr := new(net.OpError); errors.As(res.Err, &netErr) {
			m.onConnDown()
		} else {
			m.onConnUp()
		}
	} else {
		m.onConnDown()
	}
}

func (m *Manager) onConnDown() {
	m.statusLock.Lock()
	defer m.statusLock.Unlock()

	if m.status == StatusDown {
		return
	}

	m.status = StatusDown

	for _, observer := range m.observers {
		observer(m.status)
	}
}

func (m *Manager) onConnUp() {
	m.statusLock.Lock()
	defer m.statusLock.Unlock()

	if m.status == StatusUp {
		return
	}

	m.status = StatusUp

	for _, observer := range m.observers {
		observer(m.status)
	}
}
