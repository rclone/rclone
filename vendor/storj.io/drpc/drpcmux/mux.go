// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package drpcmux

import (
	"reflect"

	"github.com/zeebo/errs"

	"storj.io/drpc"
)

// Mux is an implementation of Handler to serve drpc connections to the
// appropriate Receivers registered by Descriptions.
type Mux struct {
	rpcs map[string]rpcData
}

// New constructs a new Mux.
func New() *Mux {
	return &Mux{
		rpcs: make(map[string]rpcData),
	}
}

var (
	streamType  = reflect.TypeOf((*drpc.Stream)(nil)).Elem()
	messageType = reflect.TypeOf((*drpc.Message)(nil)).Elem()
)

type rpcData struct {
	srv      interface{}
	receiver drpc.Receiver
	in1      reflect.Type
	in2      reflect.Type
}

// Register associates the rpcs described by the description in the server.
// It returns an error if there was a problem registering it.
func (m *Mux) Register(srv interface{}, desc drpc.Description) error {
	n := desc.NumMethods()
	for i := 0; i < n; i++ {
		rpc, receiver, method, ok := desc.Method(i)
		if !ok {
			return errs.New("Description returned invalid method for index %d", i)
		}
		if err := m.registerOne(srv, rpc, receiver, method); err != nil {
			return err
		}
	}
	return nil
}

// registerOne does the work to register a single rpc.
func (m *Mux) registerOne(srv interface{}, rpc string, receiver drpc.Receiver, method interface{}) error {
	data := rpcData{srv: srv, receiver: receiver}

	switch mt := reflect.TypeOf(method); {
	// unitary input, unitary output
	case mt.NumOut() == 2:
		data.in1 = mt.In(2)
		if !data.in1.Implements(messageType) {
			return errs.New("input argument not a drpc message: %v", data.in1)
		}

	// unitary input, stream output
	case mt.NumIn() == 3:
		data.in1 = mt.In(1)
		if !data.in1.Implements(messageType) {
			return errs.New("input argument not a drpc message: %v", data.in1)
		}
		data.in2 = streamType

	// stream input
	case mt.NumIn() == 2:
		data.in1 = streamType

	// code gen bug?
	default:
		return errs.New("unknown method type: %v", mt)
	}

	m.rpcs[rpc] = data
	return nil
}

// HandleRPC handles the rpc that has been requested by the stream.
func (m *Mux) HandleRPC(stream drpc.Stream, rpc string) (err error) {
	data, ok := m.rpcs[rpc]
	if !ok {
		return drpc.ProtocolError.New("unknown rpc: %q", rpc)
	}

	in := interface{}(stream)
	if data.in1 != streamType {
		msg, ok := reflect.New(data.in1.Elem()).Interface().(drpc.Message)
		if !ok {
			return drpc.InternalError.New("invalid rpc input type")
		}
		if err := stream.MsgRecv(msg); err != nil {
			return errs.Wrap(err)
		}
		in = msg
	}

	out, err := data.receiver(data.srv, stream.Context(), in, stream)
	switch {
	case err != nil:
		return errs.Wrap(err)
	case out != nil:
		return stream.MsgSend(out)
	default:
		return nil
	}
}
