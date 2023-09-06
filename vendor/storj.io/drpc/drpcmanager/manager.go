// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmanager

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"syscall"
	"time"

	"github.com/zeebo/errs"

	"storj.io/drpc"
	"storj.io/drpc/drpcdebug"
	"storj.io/drpc/drpcmetadata"
	"storj.io/drpc/drpcsignal"
	"storj.io/drpc/drpcstream"
	"storj.io/drpc/drpcwire"
	"storj.io/drpc/internal/drpcopts"
)

var managerClosed = errs.Class("manager closed")

// Options controls configuration settings for a manager.
type Options struct {
	// WriterBufferSize controls the size of the buffer that we will fill before
	// flushing. Normal writes to streams typically issue a flush explicitly.
	WriterBufferSize int

	// Reader are passed to any readers the manager creates.
	Reader drpcwire.ReaderOptions

	// Stream are passed to any streams the manager creates.
	Stream drpcstream.Options

	// SoftCancel controls if a context cancel will cause the transport to be
	// closed or, if true, a soft cancel message will be attempted if possible.
	// A soft cancel can reduce the amount of closed and dialed connections at
	// the potential cost of higher latencies if there is latent data still being
	// flushed when the cancel happens.
	SoftCancel bool

	// InactivityTimeout is the amount of time the manager will wait when creating
	// a NewServerStream. It only includes the time it is reading packets from the
	// remote client. In other words, it only includes the time that the client
	// could delay before invoking an RPC. If zero or negative, no timeout is used.
	InactivityTimeout time.Duration
}

// Manager handles the logic of managing a transport for a drpc client or server.
// It ensures that the connection is always being read from, that it is closed
// in the case that the manager is and forwarding drpc protocol messages to the
// appropriate stream.
type Manager struct {
	tr   drpc.Transport
	wr   *drpcwire.Writer
	rd   *drpcwire.Reader
	opts Options

	sem     drpcsignal.Chan      // held by the active stream
	sbuf    streamBuffer         // largest stream id created
	pkts    chan drpcwire.Packet // channel for invoke packets
	pdone   drpcsignal.Chan      // signals when a packets buffers can be reused
	sfin    chan struct{}        // shared signal for stream finished
	streams chan streamInfo      // channel to signal that a stream should start

	sigs struct {
		term   drpcsignal.Signal // set when the manager should start terminating
		stream drpcsignal.Signal // set when the manage streams goroutine is done
		read   drpcsignal.Signal // set after the goroutine reading from the transport is done
		tport  drpcsignal.Signal // set after the transport has been closed
	}
}

type streamInfo struct {
	ctx    context.Context
	stream *drpcstream.Stream
}

// New returns a new Manager for the transport.
func New(tr drpc.Transport) *Manager {
	return NewWithOptions(tr, Options{})
}

// NewWithOptions returns a new manager for the transport. It uses the provided
// options to manage details of how it uses it.
func NewWithOptions(tr drpc.Transport, opts Options) *Manager {
	m := &Manager{
		tr:   tr,
		wr:   drpcwire.NewWriter(tr, opts.WriterBufferSize),
		rd:   drpcwire.NewReaderWithOptions(tr, opts.Reader),
		opts: opts,

		pkts:    make(chan drpcwire.Packet),
		sfin:    make(chan struct{}, 1),
		streams: make(chan streamInfo),
	}

	// initialize the stream buffer
	m.sbuf.init()

	// this semaphore controls the number of concurrent streams. it MUST be 1.
	m.sem.Make(1)

	// a buffer of size 1 allows the consumer of the packet to signal it is done
	// without having to coordinate with the sender of the packet.
	m.pdone.Make(1)

	// set the internal stream options
	drpcopts.SetStreamTransport(&m.opts.Stream.Internal, m.tr)
	drpcopts.SetStreamFin(&m.opts.Stream.Internal, m.sfin)

	go m.manageReader()
	go m.manageStreams()

	return m
}

// String returns a string representation of the manager.
func (m *Manager) String() string { return fmt.Sprintf("<man %p>", m) }

func (m *Manager) log(what string, cb func() string) {
	if drpcdebug.Enabled {
		drpcdebug.Log(func() (_, _, _ string) { return m.String(), what, cb() })
	}
}

//
// helpers
//

// acquireSemaphore attempts to acquire the semaphore protecting streams. If the
// context is canceled or the manager is terminated, it returns an error.
func (m *Manager) acquireSemaphore(ctx context.Context) error {
	if err, ok := m.sigs.term.Get(); ok {
		return err
	} else if err := ctx.Err(); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-m.sigs.term.Signal():
		return m.sigs.term.Err()

	case m.sem.Get() <- struct{}{}:
		if err := m.waitForPreviousStream(ctx); err != nil {
			m.sem.Recv()
			return err
		}
		return nil
	}
}

// waitForPreviousStream will, if there was a previous stream, ensure it is Closed and
// then wait until it is in the Finished state, where it will no longer make any
// reads or writes on the transport. It exits early if the context is canceled or
// the manager is terminated.
func (m *Manager) waitForPreviousStream(ctx context.Context) (err error) {
	prev := m.sbuf.Get()
	if prev == nil {
		return nil
	}

	// if the stream is not finished yet, we need to wait for it to be
	// finished before letting the next stream to start.
	if prev.IsFinished() {
		return nil
	}

	m.log("WAIT", prev.String)

	select {
	case <-ctx.Done():
		return ctx.Err()

	case <-m.sigs.term.Signal():
		return m.sigs.term.Err()

	case <-prev.Finished():
		return nil
	}
}

// terminate puts the Manager into a terminal state and closes any resources
// that need to be closed to signal the state change.
func (m *Manager) terminate(err error) {
	if m.sigs.term.Set(err) {
		m.log("TERM", func() string { return fmt.Sprint(err) })
		m.sigs.tport.Set(m.tr.Close())
		m.sbuf.Close()
	}
}

//
// manage reader
//

// manageReader is always reading a packet and dispatching it to the appropriate
// stream or queue. It sets the read signal when it exits so that one can wait to
// ensure that no one is reading on the reader. It sets the term signal if there is
// any error reading packets.
func (m *Manager) manageReader() {
	defer m.sigs.read.Set(nil)

	var pkt drpcwire.Packet
	var err error
	var run int

	for !m.sigs.term.IsSet() {
		// if we have a run of "small" packets, drop the buffer to release
		// memory so that a burst of large packets does not cause eternally
		// large heap usage.
		if run > 10 {
			pkt.Data = nil
			run = 0
		}

		pkt, err = m.rd.ReadPacketUsing(pkt.Data[:0])
		if err != nil {
			if isConnectionReset(err) {
				err = drpc.ClosedError.Wrap(err)
			}
			m.terminate(managerClosed.Wrap(err))
			return
		}

		if len(pkt.Data) < cap(pkt.Data)/4 {
			run++
		} else {
			run = 0
		}

		m.log("READ", pkt.String)

	again:
		switch curr := m.sbuf.Get(); {
		// if the packet is for the current stream, deliver it.
		case curr != nil && pkt.ID.Stream == curr.ID():
			if err := curr.HandlePacket(pkt); err != nil {
				m.terminate(managerClosed.Wrap(err))
				return
			}

		// if an old message has been sent, just ignore it.
		case curr != nil && pkt.ID.Stream < curr.ID():

		// if any invoke sequence is being sent, close any old
		// unterminated stream and forward it to be handled.
		case pkt.Kind == drpcwire.KindInvoke || pkt.Kind == drpcwire.KindInvokeMetadata:
			if curr != nil && !curr.IsTerminated() {
				curr.Cancel(context.Canceled)
			}

			select {
			case m.pkts <- pkt:
				m.pdone.Recv()

			case <-m.sigs.term.Signal():
				return
			}

		// a non-invoke packet should be delivered to some stream
		// so we wait for a new stream to be created and try again.
		// like an invoke, we implicitly close any previous stream.
		default:
			if curr != nil && !curr.IsTerminated() {
				curr.Cancel(context.Canceled)
			}

			if !m.sbuf.Wait(curr.ID()) {
				return
			}
			goto again
		}
	}
}

//
// manage streams
//

// newStream creates a stream value with the appropriate configuration for this manager.
func (m *Manager) newStream(ctx context.Context, sid uint64, kind string) (*drpcstream.Stream, error) {
	opts := m.opts.Stream
	drpcopts.SetStreamKind(&opts.Internal, kind)

	stream := drpcstream.NewWithOptions(ctx, sid, m.wr, opts)
	select {
	case m.streams <- streamInfo{ctx: ctx, stream: stream}:
		m.sbuf.Set(stream)
		m.log("STREAM", stream.String)
		return stream, nil

	case <-m.sigs.term.Signal():
		return nil, m.sigs.term.Err()
	}
}

// manageStreams reads from the streams channel for stream infos and runs the
// manageStream function on them.
func (m *Manager) manageStreams() {
	defer m.sigs.stream.Set(nil)

	for {
		select {
		case si := <-m.streams:
			m.manageStream(si.ctx, si.stream)

		case <-m.sigs.term.Signal():
			return
		}
	}
}

// manageStream watches the context and the stream and returns when the stream is
// finished, canceling the stream if the context is canceled.
func (m *Manager) manageStream(ctx context.Context, stream *drpcstream.Stream) {
	select {
	case <-m.sigs.term.Signal():
		err := m.sigs.term.Err()
		if errors.Is(err, io.EOF) {
			err = context.Canceled
		}
		stream.Cancel(err)
		<-m.sfin
		m.sem.Recv()

	case <-m.sfin:
		m.sem.Recv()

	case <-ctx.Done():
		m.log("CANCEL", stream.String)

		if m.opts.SoftCancel {
			// allow a new stream to begin.
			m.sem.Recv()

			// attempt to send the soft cancel. if it fails or if the stream is busy
			// sending something else, then we have to hard cancel.
			if busy, err := stream.SendCancel(ctx.Err()); err != nil {
				m.terminate(err)
			} else if busy {
				m.terminate(ctx.Err())
			}
			stream.Cancel(ctx.Err())

			// wait for the stream to signal that it is finished.
			<-m.sfin
		} else {
			// If the stream isn't already finished, we have to terminate the transport
			// to do an active cancel. If it is already finished, there is no need.
			if !stream.Cancel(ctx.Err()) {
				m.log("UNFIN", stream.String)
				m.terminate(ctx.Err())
			} else {
				m.log("CLEAN", stream.String)
			}

			// wait for the stream to signal that it is finished.
			<-m.sfin

			// allow a new stream to begin.
			m.sem.Recv()
		}
	}
}

//
// exported interface
//

// Closed returns a channel that is closed once the manager is closed.
func (m *Manager) Closed() <-chan struct{} {
	return m.sigs.term.Signal()
}

// Unblocked returns a channel that is closed when the manager is no longer blocked
// from creating a new stream due to a previous stream's soft cancel. It should not
// be called concurrently with NewClientStream or NewServerStream and the return
// result is only valid until the next call to NewClientStream or NewServerStream.
func (m *Manager) Unblocked() <-chan struct{} {
	if prev := m.sbuf.Get(); prev != nil {
		return prev.Context().Done()
	}
	return closedCh
}

// Close closes the transport the manager is using.
func (m *Manager) Close() error {
	m.terminate(managerClosed.New("Close called"))

	m.sigs.stream.Wait()
	m.sigs.read.Wait()
	m.sigs.tport.Wait()

	return m.sigs.tport.Err()
}

// NewClientStream starts a stream on the managed transport for use by a client.
func (m *Manager) NewClientStream(ctx context.Context) (stream *drpcstream.Stream, err error) {
	if err := m.acquireSemaphore(ctx); err != nil {
		return nil, err
	}

	return m.newStream(ctx, m.sbuf.Get().ID()+1, "cli")
}

// NewServerStream starts a stream on the managed transport for use by a server. It does
// this by waiting for the client to issue an invoke message and returning the details.
func (m *Manager) NewServerStream(ctx context.Context) (stream *drpcstream.Stream, rpc string, err error) {
	if err := m.acquireSemaphore(ctx); err != nil {
		return nil, "", err
	}
	defer func() {
		if err != nil {
			m.sem.Recv()
		}
	}()

	var meta map[string]string
	var metaID uint64
	var timeoutCh <-chan time.Time

	// set up the timeout on the context if necessary.
	if timeout := m.opts.InactivityTimeout; timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		timeoutCh = timer.C
	}

	for {
		select {
		case <-timeoutCh:
			return nil, "", context.DeadlineExceeded

		case <-ctx.Done():
			return nil, "", ctx.Err()

		case <-m.sigs.term.Signal():
			return nil, "", m.sigs.term.Err()

		case pkt := <-m.pkts:
			switch pkt.Kind {
			// keep track of any metadata being sent before an invoke so that we can
			// include it if the stream id matches the eventual invoke.
			case drpcwire.KindInvokeMetadata:
				meta, err = drpcmetadata.Decode(pkt.Data)
				m.pdone.Send()

				if err != nil {
					return nil, "", err
				}
				metaID = pkt.ID.Stream

			case drpcwire.KindInvoke:
				rpc = string(pkt.Data)
				m.pdone.Send()

				if metaID == pkt.ID.Stream {
					ctx = drpcmetadata.AddPairs(ctx, meta)
				}

				stream, err := m.newStream(ctx, pkt.ID.Stream, "srv")
				return stream, rpc, err

			default:
				// this should never happen, but defensive.
				m.pdone.Send()
			}
		}
	}
}

func isConnectionReset(err error) bool {
	var operr *net.OpError
	if !errors.As(err, &operr) {
		return false
	}
	if errors.Is(operr.Err, syscall.ECONNRESET) {
		return true
	}
	msg := strings.ToLower(operr.Err.Error())
	if strings.Contains(msg, "connection reset by peer") {
		return true
	}
	if strings.Contains(msg, "connection was forcibly closed by the remote host") {
		return true
	}
	if strings.Contains(msg, strings.ToLower(syscall.ECONNRESET.Error())) {
		return true
	}
	return false
}
