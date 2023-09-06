package sftp

import (
	"context"
	"errors"
	"io"
	"path"
	"path/filepath"
	"strconv"
	"sync"
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
	Handlers Handlers

	*serverConn
	pktMgr *packetManager

	startDirectory string

	mu           sync.RWMutex
	handleCount  int
	openRequests map[string]*Request
}

// A RequestServerOption is a function which applies configuration to a RequestServer.
type RequestServerOption func(*RequestServer)

// WithRSAllocator enable the allocator.
// After processing a packet we keep in memory the allocated slices
// and we reuse them for new packets.
// The allocator is experimental
func WithRSAllocator() RequestServerOption {
	return func(rs *RequestServer) {
		alloc := newAllocator()
		rs.pktMgr.alloc = alloc
		rs.conn.alloc = alloc
	}
}

// WithStartDirectory sets a start directory to use as base for relative paths.
// If unset the default is "/"
func WithStartDirectory(startDirectory string) RequestServerOption {
	return func(rs *RequestServer) {
		rs.startDirectory = cleanPath(startDirectory)
	}
}

// NewRequestServer creates/allocates/returns new RequestServer.
// Normally there will be one server per user-session.
func NewRequestServer(rwc io.ReadWriteCloser, h Handlers, options ...RequestServerOption) *RequestServer {
	svrConn := &serverConn{
		conn: conn{
			Reader:      rwc,
			WriteCloser: rwc,
		},
	}
	rs := &RequestServer{
		Handlers: h,

		serverConn: svrConn,
		pktMgr:     newPktMgr(svrConn),

		startDirectory: "/",

		openRequests: make(map[string]*Request),
	}

	for _, o := range options {
		o(rs)
	}
	return rs
}

// New Open packet/Request
func (rs *RequestServer) nextRequest(r *Request) string {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	rs.handleCount++

	r.handle = strconv.Itoa(rs.handleCount)
	rs.openRequests[r.handle] = r

	return r.handle
}

// Returns Request from openRequests, bool is false if it is missing.
//
// The Requests in openRequests work essentially as open file descriptors that
// you can do different things with. What you are doing with it are denoted by
// the first packet of that type (read/write/etc).
func (rs *RequestServer) getRequest(handle string) (*Request, bool) {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	r, ok := rs.openRequests[handle]
	return r, ok
}

// Close the Request and clear from openRequests map
func (rs *RequestServer) closeRequest(handle string) error {
	rs.mu.Lock()
	defer rs.mu.Unlock()

	if r, ok := rs.openRequests[handle]; ok {
		delete(rs.openRequests, handle)
		return r.close()
	}

	return EBADF
}

// Close the read/write/closer to trigger exiting the main server loop
func (rs *RequestServer) Close() error { return rs.conn.Close() }

func (rs *RequestServer) serveLoop(pktChan chan<- orderedRequest) error {
	defer close(pktChan) // shuts down sftpServerWorkers

	var err error
	var pkt requestPacket
	var pktType uint8
	var pktBytes []byte

	for {
		pktType, pktBytes, err = rs.serverConn.recvPacket(rs.pktMgr.getNextOrderID())
		if err != nil {
			// we don't care about releasing allocated pages here, the server will quit and the allocator freed
			return err
		}

		pkt, err = makePacket(rxPacket{fxp(pktType), pktBytes})
		if err != nil {
			switch {
			case errors.Is(err, errUnknownExtendedPacket):
				// do nothing
			default:
				debug("makePacket err: %v", err)
				rs.conn.Close() // shuts down recvPacket
				return err
			}
		}

		pktChan <- rs.pktMgr.newOrderedRequest(pkt)
	}
}

// Serve requests for user session
func (rs *RequestServer) Serve() error {
	defer func() {
		if rs.pktMgr.alloc != nil {
			rs.pktMgr.alloc.Free()
		}
	}()

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

	err := rs.serveLoop(pktChan)

	wg.Wait() // wait for all workers to exit

	rs.mu.Lock()
	defer rs.mu.Unlock()

	// make sure all open requests are properly closed
	// (eg. possible on dropped connections, client crashes, etc.)
	for handle, req := range rs.openRequests {
		if err == io.EOF {
			err = io.ErrUnexpectedEOF
		}
		req.transferError(err)

		delete(rs.openRequests, handle)
		req.close()
	}

	return err
}

func (rs *RequestServer) packetWorker(ctx context.Context, pktChan chan orderedRequest) error {
	for pkt := range pktChan {
		orderID := pkt.orderID()
		if epkt, ok := pkt.requestPacket.(*sshFxpExtendedPacket); ok {
			if epkt.SpecificPacket != nil {
				pkt.requestPacket = epkt.SpecificPacket
			}
		}

		var rpkt responsePacket
		switch pkt := pkt.requestPacket.(type) {
		case *sshFxInitPacket:
			rpkt = &sshFxVersionPacket{Version: sftpProtocolVersion, Extensions: sftpExtensions}
		case *sshFxpClosePacket:
			handle := pkt.getHandle()
			rpkt = statusFromError(pkt.ID, rs.closeRequest(handle))
		case *sshFxpRealpathPacket:
			var realPath string
			var err error

			switch pather := rs.Handlers.FileList.(type) {
			case RealPathFileLister:
				realPath, err = pather.RealPath(pkt.getPath())
			case legacyRealPathFileLister:
				realPath = pather.RealPath(pkt.getPath())
			default:
				realPath = cleanPathWithBase(rs.startDirectory, pkt.getPath())
			}
			if err != nil {
				rpkt = statusFromError(pkt.ID, err)
			} else {
				rpkt = cleanPacketPath(pkt, realPath)
			}
		case *sshFxpOpendirPacket:
			request := requestFromPacket(ctx, pkt, rs.startDirectory)
			handle := rs.nextRequest(request)
			rpkt = request.opendir(rs.Handlers, pkt)
			if _, ok := rpkt.(*sshFxpHandlePacket); !ok {
				// if we return an error we have to remove the handle from the active ones
				rs.closeRequest(handle)
			}
		case *sshFxpOpenPacket:
			request := requestFromPacket(ctx, pkt, rs.startDirectory)
			handle := rs.nextRequest(request)
			rpkt = request.open(rs.Handlers, pkt)
			if _, ok := rpkt.(*sshFxpHandlePacket); !ok {
				// if we return an error we have to remove the handle from the active ones
				rs.closeRequest(handle)
			}
		case *sshFxpFstatPacket:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt.ID, EBADF)
			} else {
				request = &Request{
					Method:   "Stat",
					Filepath: cleanPathWithBase(rs.startDirectory, request.Filepath),
				}
				rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
			}
		case *sshFxpFsetstatPacket:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt.ID, EBADF)
			} else {
				request = &Request{
					Method:   "Setstat",
					Filepath: cleanPathWithBase(rs.startDirectory, request.Filepath),
				}
				rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
			}
		case *sshFxpExtendedPacketPosixRename:
			request := &Request{
				Method:   "PosixRename",
				Filepath: cleanPathWithBase(rs.startDirectory, pkt.Oldpath),
				Target:   cleanPathWithBase(rs.startDirectory, pkt.Newpath),
			}
			rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
		case *sshFxpExtendedPacketStatVFS:
			request := &Request{
				Method:   "StatVFS",
				Filepath: cleanPathWithBase(rs.startDirectory, pkt.Path),
			}
			rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
		case hasHandle:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt.id(), EBADF)
			} else {
				rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
			}
		case hasPath:
			request := requestFromPacket(ctx, pkt, rs.startDirectory)
			rpkt = request.call(rs.Handlers, pkt, rs.pktMgr.alloc, orderID)
			request.close()
		default:
			rpkt = statusFromError(pkt.id(), ErrSSHFxOpUnsupported)
		}

		rs.pktMgr.readyPacket(
			rs.pktMgr.newOrderedResponse(rpkt, orderID))
	}
	return nil
}

// clean and return name packet for file
func cleanPacketPath(pkt *sshFxpRealpathPacket, realPath string) responsePacket {
	return &sshFxpNamePacket{
		ID: pkt.id(),
		NameAttrs: []*sshFxpNameAttr{
			{
				Name:     realPath,
				LongName: realPath,
				Attrs:    emptyFileStat,
			},
		},
	}
}

// Makes sure we have a clean POSIX (/) absolute path to work with
func cleanPath(p string) string {
	return cleanPathWithBase("/", p)
}

func cleanPathWithBase(base, p string) string {
	p = filepath.ToSlash(filepath.Clean(p))
	if !path.IsAbs(p) {
		return path.Join(base, p)
	}
	return p
}
