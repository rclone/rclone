// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

package multidial

import (
	"context"
	"errors"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/zeebo/errs"
	"golang.org/x/sync/errgroup"

	"storj.io/common/sync2"
)

var (
	// Error is a generic error of this package.
	Error = errs.Class("multidial")
	// ErrConnClosed happens when a request is made against a closed subconn.
	ErrConnClosed = errs.Class("conn closed")
)

const (
	maxIncompleteRequestQueueSize = 5
)

type requestType int

const (
	// the below requestTypes are for calls made to the conn.
	typeRead             requestType = 1
	typeWrite            requestType = 2
	typeSetDeadline      requestType = 3
	typeSetReadDeadline  requestType = 4
	typeSetWriteDeadline requestType = 5

	// the "choose" request type is for when one of the two connections is selected
	// and chosen to be the one that will continue to be interacted with.
	// this request type causes the manager to release ownership of the connection.
	typeChoose requestType = 6
)

// connResponse encapsulates all the possible return values from one of the above
// requestTypes.
type connResponse struct {
	// typeRead and typeWrite return (n, err), so they use N and Err.
	N int
	// typeSet.*Deadline return err, so they use Err.
	Err error
	// typeChoose returns the chosen connection, indicating that ownership has been
	// released from its manager.
	Conn net.Conn

	// Source is set on all responses.
	Source *connDetails
}

// connRequest encapsulates all the possible arguments to a given request.
type connRequest struct {
	// Type is the type of the request.
	Type requestType

	// typeRead and typeWrite take a []byte, so they use Buf.
	Buf []byte
	// typeSet.*Deadline take a time.Time, so they use T.
	T time.Time

	// Response is set on all requests and is the channel the response should
	// be sent down.
	Response chan connResponse
}

type connDetails struct {
	conn        atomicConn
	connClaimed int32
	cancel      func()
	reqs        *sync2.ReceiverClosableChan[connRequest]
}

func (cd *connDetails) init(ctx context.Context) context.Context {
	ctx, cd.cancel = context.WithCancel(ctx)
	cd.reqs = sync2.MakeReceiverClosableChan[connRequest](maxIncompleteRequestQueueSize)
	return ctx
}

type setup struct {
	network      string
	address      string
	conn1, conn2 connDetails
}

func newSetup(ctx context.Context, m *Multidialer, network string, address string) *setup {
	s := &setup{
		network: network,
		address: address,
	}
	go s.conn1.manage(s.conn1.init(ctx), m.dialer1, network, address)
	go s.conn2.manage(s.conn2.init(ctx), m.dialer2, network, address)
	return s
}

func (cd *connDetails) requestOne(req connRequest) {
	if !cd.reqs.BlockingSend(req) {
		req.Response <- connResponse{
			Err:    ErrConnClosed.New("op"),
			Source: cd,
		}
	}
}

func (s *setup) request(req1, req2 connRequest) {
	s.conn1.requestOne(req1)
	s.conn2.requestOne(req2)
}

func (s *setup) setDeadline(t time.Time, typ requestType) (err error) {
	req := connRequest{
		Response: make(chan connResponse, 2),
		Type:     typ,
		T:        t,
	}
	s.request(req, req)

	for i := 0; i < 2; i++ {
		err = (<-req.Response).Err
		if err == nil {
			return nil
		}
	}
	return err
}

func (s *setup) SetDeadline(t time.Time) (err error) {
	return s.setDeadline(t, typeSetDeadline)
}

func (s *setup) SetReadDeadline(t time.Time) (err error) {
	return s.setDeadline(t, typeSetReadDeadline)
}

func (s *setup) SetWriteDeadline(t time.Time) (err error) {
	return s.setDeadline(t, typeSetWriteDeadline)
}

func (s *setup) Write(p []byte) (n int, err error) {
	req := connRequest{
		Response: make(chan connResponse, 2),
		Type:     typeWrite,
		Buf:      append([]byte(nil), p...),
	}
	s.request(req, req)

	for i := 0; i < 2; i++ {
		answer := <-req.Response
		n, err = answer.N, answer.Err
		if err == nil {
			return n, nil
		}
	}
	return n, err
}

// Read will read from whichever connection is fastest to respond. If
// we have selected a connection, it will return the connection.
// Note that if (*setup).Close and (*setup).Read are called
// concurrently, Read may return an unclosed connection! It is
// (*conn).Read's responsibility to handle this case.
func (s *setup) Read(p []byte) (n int, conn net.Conn, err error) {
	p1 := make([]byte, len(p))
	p2 := make([]byte, len(p))
	resp := make(chan connResponse, 2)

	s.request(connRequest{
		Response: resp,
		Type:     typeRead,
		Buf:      p1,
	}, connRequest{
		Response: resp,
		Type:     typeRead,
		Buf:      p2,
	})

	for i := 0; i < 2; i++ {
		answer := <-resp
		n, err = answer.N, answer.Err
		if err != nil {
			// in the case that errors.Is(err, io.EOF), it might be
			// because the server side debounced us and just closed
			// cleanly.
			// it's possible that receiving an io.EOF is actually
			// the expected application protocol behavior, so how
			// do we distinguish?
			// answer: we don't! every use of this multidialer for
			// us will be over an encrypted stream that won't have
			// application behavior that just closes. so, it's safe
			// to treat an io.EOF here on this setup read as a
			// reason to wait for the other stream.
			continue
		}

		switch answer.Source {
		case &s.conn1:
			copy(p, p1[:n])
		case &s.conn2:
			copy(p, p2[:n])
		default:
			panic("unreachable")
		}

		selection := make(chan connResponse, 1)
		answer.Source.requestOne(connRequest{
			Response: selection,
			Type:     typeChoose,
		})
		selectionAnswer := <-selection
		if selectionAnswer.Err != nil {
			// well, the read succeeded, so let's at least return that.
			return n, nil, err
		}

		return n, selectionAnswer.Conn, err
	}
	return n, nil, err
}

func (cd *connDetails) Close() error {
	cd.cancel()
	if conn, ok := cd.conn.Load(); ok && atomic.LoadInt32(&cd.connClaimed) == 0 {
		return conn.Close()
	}
	// note that a Read may be happening concurrently, and we might be in the
	// middle of typeChoose. in this case, the connection that Read returns
	// will not be closed. See the comment on (*setup).Read for more details.
	return nil
}

func (s *setup) Close() error {
	var eg errgroup.Group
	eg.Go(s.conn1.Close)
	eg.Go(s.conn2.Close)
	return eg.Wait()
}

func (cd *connDetails) manage(ctx context.Context, dialer DialFunc, network, address string) {
	var connErr error
	defer func() {
		// ha ha! the receiver closes the chan! take that rob pike!
		drain := cd.reqs.StopReceiving()
		if connErr == nil {
			connErr = ErrConnClosed.New("teardown")
		}
		// drain the incoming requests.
		for _, req := range drain {
			req.Response <- connResponse{
				Err:    connErr,
				Source: cd,
			}
		}
	}()
	conn, err := dialer(ctx, network, address)
	if err != nil {
		connErr = err
		return
	}
	defer func() {
		if conn != nil {
			connErr = errs.Combine(connErr, conn.Close())
		}
	}()

	cd.conn.Store(conn)
	// we need to check that ctx is closed, now that we called conn.Store
	// to avoid a race with the cancel() close() dance that setup.Close
	// does. fortunately, cd.reqs.Receive below does this for us.

	for {
		// invariant: if some sort of permanent error happens,
		// set connErr and return.
		req, err := cd.reqs.Receive(ctx)
		if err != nil {
			connErr = err
			return
		}
		switch req.Type {
		case typeRead:
			n, err := conn.Read(req.Buf)
			req.Response <- connResponse{
				N:      n,
				Err:    err,
				Source: cd,
			}
			if err != nil {
				if !errors.Is(err, io.EOF) {
					connErr = err
				}
				return
			}
		case typeWrite:
			n, err := conn.Write(req.Buf)
			req.Response <- connResponse{
				N:      n,
				Err:    err,
				Source: cd,
			}
			if err != nil {
				connErr = err
				return
			}
		case typeSetDeadline:
			connErr = conn.SetDeadline(req.T)
			req.Response <- connResponse{
				Err:    connErr,
				Source: cd,
			}
			if connErr != nil {
				return
			}
		case typeSetReadDeadline:
			connErr = conn.SetReadDeadline(req.T)
			req.Response <- connResponse{
				Err:    connErr,
				Source: cd,
			}
			if connErr != nil {
				return
			}
		case typeSetWriteDeadline:
			connErr = conn.SetWriteDeadline(req.T)
			req.Response <- connResponse{
				Err:    connErr,
				Source: cd,
			}
			if connErr != nil {
				return
			}
		case typeChoose:
			atomic.StoreInt32(&cd.connClaimed, 1)
			req.Response <- connResponse{
				Source: cd,
				Conn:   conn,
			}
			conn = nil
			return
		default:
			connErr = Error.New("unknown request type")
			return
		}
	}
}
