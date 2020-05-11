// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcconn

import (
	"context"

	"github.com/gogo/protobuf/proto"
	"github.com/zeebo/errs"

	"storj.io/drpc"
	"storj.io/drpc/drpcmanager"
	"storj.io/drpc/drpcmetadata"
	"storj.io/drpc/drpcstream"
	"storj.io/drpc/drpcwire"
)

// Options controls configuration settings for a conn.
type Options struct {
	// Manager controls the options we pass to the manager of this conn.
	Manager drpcmanager.Options
}

// Conn is a drpc client connection.
type Conn struct {
	tr  drpc.Transport
	man *drpcmanager.Manager
}

var _ drpc.Conn = (*Conn)(nil)

// New returns a conn that uses the transport for reads and writes.
func New(tr drpc.Transport) *Conn {
	return NewWithOptions(tr, Options{})
}

// NewWithOptions returns a conn that uses the transport for reads and writes.
// The Options control details of how the conn operates.
func NewWithOptions(tr drpc.Transport, opts Options) *Conn {
	return &Conn{
		tr:  tr,
		man: drpcmanager.NewWithOptions(tr, opts.Manager),
	}
}

// Transport returns the transport the conn is using.
func (c *Conn) Transport() drpc.Transport {
	return c.tr
}

// Closed returns true if the connection is already closed.
func (c *Conn) Closed() bool {
	return c.man.Closed()
}

// Close closes the connection.
func (c *Conn) Close() (err error) {
	return c.man.Close()
}

// Invoke issues the rpc on the transport serializing in, waits for a response, and
// deserializes it into out. Only one Invoke or Stream may be open at a time.
func (c *Conn) Invoke(ctx context.Context, rpc string, in, out drpc.Message) (err error) {
	defer mon.Task()(&ctx)(&err)
	defer mon.TaskNamed("invoke" + rpc)(&ctx)(&err)
	mon.Event("outgoing_requests")
	mon.Event("outgoing_invokes")

	var metadata []byte
	if md, ok := drpcmetadata.Get(ctx); ok {
		metadata, err = drpcmetadata.Encode(metadata, md)
		if err != nil {
			return err
		}
	}

	data, err := proto.Marshal(in)
	if err != nil {
		return errs.Wrap(err)
	}

	stream, err := c.man.NewClientStream(ctx)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, stream.Close()) }()

	if err := c.doInvoke(stream, []byte(rpc), data, metadata, out); err != nil {
		return err
	}
	return nil
}

func (c *Conn) doInvoke(stream *drpcstream.Stream, rpc, data []byte, metadata []byte, out drpc.Message) (err error) {
	if len(metadata) > 0 {
		if err := stream.RawWrite(drpcwire.KindInvokeMetadata, metadata); err != nil {
			return err
		}
	}
	if err := stream.RawWrite(drpcwire.KindInvoke, rpc); err != nil {
		return err
	}
	if err := stream.RawWrite(drpcwire.KindMessage, data); err != nil {
		return err
	}
	if err := stream.CloseSend(); err != nil {
		return err
	}
	if err := stream.MsgRecv(out); err != nil {
		return err
	}
	return nil
}

// NewStream begins a streaming rpc on the connection. Only one Invoke or Stream may
// be open at a time.
func (c *Conn) NewStream(ctx context.Context, rpc string) (_ drpc.Stream, err error) {
	defer mon.Task()(&ctx)(&err)
	defer mon.TaskNamed("stream" + rpc)(&ctx)(&err)
	mon.Event("outgoing_requests")
	mon.Event("outgoing_streams")

	var metadata []byte
	if md, ok := drpcmetadata.Get(ctx); ok {
		metadata, err = drpcmetadata.Encode(metadata, md)
		if err != nil {
			return nil, err
		}
	}

	stream, err := c.man.NewClientStream(ctx)
	if err != nil {
		return nil, err
	}

	if err := c.doNewStream(stream, []byte(rpc), metadata); err != nil {
		return nil, errs.Combine(err, stream.Close())
	}
	return stream, nil
}

func (c *Conn) doNewStream(stream *drpcstream.Stream, rpc []byte, metadata []byte) error {
	if len(metadata) > 0 {
		if err := stream.RawWrite(drpcwire.KindInvokeMetadata, metadata); err != nil {
			return err
		}
	}
	if err := stream.RawWrite(drpcwire.KindInvoke, rpc); err != nil {
		return err
	}
	if err := stream.RawFlush(); err != nil {
		return err
	}
	return nil
}
