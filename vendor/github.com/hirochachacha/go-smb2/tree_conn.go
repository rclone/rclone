package smb2

import (
	"context"
	"fmt"

	. "github.com/hirochachacha/go-smb2/internal/smb2"
)

type treeConn struct {
	*session
	treeId     uint32
	shareFlags uint32

	// path string
	// shareType  uint8
	// capabilities uint32
	// maximalAccess uint32
}

func treeConnect(s *session, path string, flags uint16, ctx context.Context) (*treeConn, error) {
	req := &TreeConnectRequest{
		Flags: flags,
		Path:  path,
	}

	req.CreditCharge = 1

	rr, err := s.send(req, ctx)
	if err != nil {
		return nil, err
	}

	pkt, err := s.recv(rr)
	if err != nil {
		return nil, err
	}

	res, err := accept(SMB2_TREE_CONNECT, pkt)
	if err != nil {
		return nil, err
	}

	r := TreeConnectResponseDecoder(res)
	if r.IsInvalid() {
		return nil, &InvalidResponseError{"broken tree connect response format"}
	}

	tc := &treeConn{
		session:    s,
		treeId:     PacketCodec(pkt).TreeId(),
		shareFlags: r.ShareFlags(),
		// path:    path,
		// shareType:  r.ShareType(),
		// capabilities: r.Capabilities(),
		// maximalAccess: r.MaximalAccess(),
	}

	return tc, nil
}

func (tc *treeConn) disconnect(ctx context.Context) error {
	req := new(TreeDisconnectRequest)

	req.CreditCharge = 1

	res, err := tc.sendRecv(SMB2_TREE_DISCONNECT, req, ctx)
	if err != nil {
		return err
	}

	r := TreeDisconnectResponseDecoder(res)
	if r.IsInvalid() {
		return &InvalidResponseError{"broken tree disconnect response format"}
	}

	return nil
}

func (tc *treeConn) sendRecv(cmd uint16, req Packet, ctx context.Context) (res []byte, err error) {
	rr, err := tc.send(req, ctx)
	if err != nil {
		return nil, err
	}

	pkt, err := tc.recv(rr)
	if err != nil {
		return nil, err
	}

	return accept(cmd, pkt)
}

func (tc *treeConn) send(req Packet, ctx context.Context) (rr *requestResponse, err error) {
	return tc.sendWith(req, tc, ctx)
}

func (tc *treeConn) recv(rr *requestResponse) (pkt []byte, err error) {
	pkt, err = tc.session.recv(rr)
	if err != nil {
		return nil, err
	}
	if rr.asyncId != 0 {
		if asyncId := PacketCodec(pkt).AsyncId(); asyncId != rr.asyncId {
			return nil, &InvalidResponseError{fmt.Sprintf("expected async id: %v, got %v", rr.asyncId, asyncId)}
		}
	} else {
		if treeId := PacketCodec(pkt).TreeId(); treeId != tc.treeId {
			return nil, &InvalidResponseError{fmt.Sprintf("expected tree id: %v, got %v", tc.treeId, treeId)}
		}
	}
	return pkt, err
}
