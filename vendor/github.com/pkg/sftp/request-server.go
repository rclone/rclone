package sftp

import (
	"context"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/pkg/errors"
)

var maxTxPacket uint32 = 1 << 15

// Handlers contains the 4 SFTP server request handlers.
type Handlers struct {
	FileGet  FileReader
	FilePut  FileWriter
	FileCmd  FileCmder
	FileList FileLister
}

// RequestServer abstracts the sftp protocol with an http request-like protocol
type RequestServer struct {
	*serverConn
	Handlers        Handlers
	pktMgr          *packetManager
	openRequests    map[string]*Request
	openRequestLock sync.RWMutex
	handleCount     int
}

// NewRequestServer creates/allocates/returns new RequestServer.
// Normally there there will be one server per user-session.
func NewRequestServer(rwc io.ReadWriteCloser, h Handlers) *RequestServer {
	svrConn := &serverConn{
		conn: conn{
			Reader:      rwc,
			WriteCloser: rwc,
		},
	}
	return &RequestServer{
		serverConn:   svrConn,
		Handlers:     h,
		pktMgr:       newPktMgr(svrConn),
		openRequests: make(map[string]*Request),
	}
}

// New Open packet/Request
func (rs *RequestServer) nextRequest(r *Request) string {
	rs.openRequestLock.Lock()
	defer rs.openRequestLock.Unlock()
	rs.handleCount++
	handle := strconv.Itoa(rs.handleCount)
	r.handle = handle
	rs.openRequests[handle] = r
	return handle
}

// Returns Request from openRequests, bool is false if it is missing.
//
// The Requests in openRequests work essentially as open file descriptors that
// you can do different things with. What you are doing with it are denoted by
// the first packet of that type (read/write/etc).
func (rs *RequestServer) getRequest(handle string) (*Request, bool) {
	rs.openRequestLock.RLock()
	defer rs.openRequestLock.RUnlock()
	r, ok := rs.openRequests[handle]
	return r, ok
}

// Close the Request and clear from openRequests map
func (rs *RequestServer) closeRequest(handle string) error {
	rs.openRequestLock.Lock()
	defer rs.openRequestLock.Unlock()
	if r, ok := rs.openRequests[handle]; ok {
		delete(rs.openRequests, handle)
		return r.close()
	}
	return syscall.EBADF
}

// Close the read/write/closer to trigger exiting the main server loop
func (rs *RequestServer) Close() error { return rs.conn.Close() }

// Serve requests for user session
func (rs *RequestServer) Serve() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var wg sync.WaitGroup
	runWorker := func(ch chan orderedRequest) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := rs.packetWorker(ctx, ch); err != nil {
				rs.conn.Close() // shuts down recvPacket
			}
		}()
	}
	pktChan := rs.pktMgr.workerChan(runWorker)

	var err error
	var pkt requestPacket
	var pktType uint8
	var pktBytes []byte
	for {
		pktType, pktBytes, err = rs.recvPacket()
		if err != nil {
			break
		}

		pkt, err = makePacket(rxPacket{fxp(pktType), pktBytes})
		if err != nil {
			switch errors.Cause(err) {
			case errUnknownExtendedPacket:
				if err := rs.serverConn.sendError(pkt, ErrSshFxOpUnsupported); err != nil {
					debug("failed to send err packet: %v", err)
					rs.conn.Close() // shuts down recvPacket
					break
				}
			default:
				debug("makePacket err: %v", err)
				rs.conn.Close() // shuts down recvPacket
				break
			}
		}

		pktChan <- rs.pktMgr.newOrderedRequest(pkt)
	}

	close(pktChan) // shuts down sftpServerWorkers
	wg.Wait()      // wait for all workers to exit

	// make sure all open requests are properly closed
	// (eg. possible on dropped connections, client crashes, etc.)
	for handle, req := range rs.openRequests {
		delete(rs.openRequests, handle)
		req.close()
	}

	return err
}

func (rs *RequestServer) packetWorker(
	ctx context.Context, pktChan chan orderedRequest,
) error {
	for pkt := range pktChan {
		var rpkt responsePacket
		switch pkt := pkt.requestPacket.(type) {
		case *sshFxInitPacket:
			rpkt = sshFxVersionPacket{Version: sftpProtocolVersion}
		case *sshFxpClosePacket:
			handle := pkt.getHandle()
			rpkt = statusFromError(pkt, rs.closeRequest(handle))
		case *sshFxpRealpathPacket:
			rpkt = cleanPacketPath(pkt)
		case *sshFxpOpendirPacket:
			request := requestFromPacket(ctx, pkt)
			rs.nextRequest(request)
			rpkt = request.opendir(rs.Handlers, pkt)
		case *sshFxpOpenPacket:
			request := requestFromPacket(ctx, pkt)
			rs.nextRequest(request)
			rpkt = request.open(rs.Handlers, pkt)
		case *sshFxpFstatPacket:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt, syscall.EBADF)
			} else {
				request = NewRequest("Stat", request.Filepath)
				rpkt = request.call(rs.Handlers, pkt)
			}
		case hasHandle:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt, syscall.EBADF)
			} else {
				rpkt = request.call(rs.Handlers, pkt)
			}
		case hasPath:
			request := requestFromPacket(ctx, pkt)
			rpkt = request.call(rs.Handlers, pkt)
			request.close()
		default:
			return errors.Errorf("unexpected packet type %T", pkt)
		}

		rs.pktMgr.readyPacket(
			rs.pktMgr.newOrderedResponse(rpkt, pkt.orderId()))
	}
	return nil
}

// clean and return name packet for file
func cleanPacketPath(pkt *sshFxpRealpathPacket) responsePacket {
	path := cleanPath(pkt.getPath())
	return &sshFxpNamePacket{
		ID: pkt.id(),
		NameAttrs: []sshFxpNameAttr{{
			Name:     path,
			LongName: path,
			Attrs:    emptyFileStat,
		}},
	}
}

// Makes sure we have a clean POSIX (/) absolute path to work with
func cleanPath(p string) string {
	p = filepath.ToSlash(p)
	if !filepath.IsAbs(p) {
		p = "/" + p
	}
	return path.Clean(p)
}
