package sftp

import (
	"context"
	"io"
	"os"
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
	rs.openRequests[handle] = r
	return handle
}

// Returns Request from openRequests, bool is false if it is missing
// If the method is different, save/return a new Request w/ that Method.
//
// The Requests in openRequests work essentially as open file descriptors that
// you can do different things with. What you are doing with it are denoted by
// the first packet of that type (read/write/etc). We create a new Request when
// it changes to set the request.Method attribute in a thread safe way.
func (rs *RequestServer) getRequest(handle, method string) (*Request, bool) {
	rs.openRequestLock.RLock()
	r, ok := rs.openRequests[handle]
	rs.openRequestLock.RUnlock()
	if !ok || r.Method == method {
		return r, ok
	}
	// if we make it here we need to replace the request
	rs.openRequestLock.Lock()
	defer rs.openRequestLock.Unlock()
	r, ok = rs.openRequests[handle]
	if !ok || r.Method == method { // re-check needed b/c lock race
		return r, ok
	}
	r = r.copy()
	r.Method = method
	rs.openRequests[handle] = r
	return r, ok
}

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
			rpkt = request.call(rs.Handlers, pkt)
			if stat, ok := rpkt.(*sshFxpStatResponse); ok {
				if stat.info.IsDir() {
					handle := rs.nextRequest(request)
					rpkt = sshFxpHandlePacket{ID: pkt.id(), Handle: handle}
				} else {
					rpkt = statusFromError(pkt, &os.PathError{
						Path: request.Filepath, Err: syscall.ENOTDIR})
				}
			}
		case *sshFxpOpenPacket:
			request := requestFromPacket(ctx, pkt)
			handle := rs.nextRequest(request)
			rpkt = sshFxpHandlePacket{ID: pkt.id(), Handle: handle}
			if pkt.hasPflags(ssh_FXF_CREAT) {
				if p := request.call(rs.Handlers, pkt); !statusOk(p) {
					rpkt = p // if error in write, return it
				}
			}
		case hasHandle:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle, requestMethod(pkt))
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

// True is responsePacket is an OK status packet
func statusOk(rpkt responsePacket) bool {
	p, ok := rpkt.(sshFxpStatusPacket)
	return ok && p.StatusError.Code == ssh_FX_OK
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
