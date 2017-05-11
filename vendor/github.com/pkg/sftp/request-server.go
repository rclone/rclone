package sftp

import (
	"encoding"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"

	"github.com/pkg/errors"
)

var maxTxPacket uint32 = 1 << 15

type handleHandler func(string) string

// Handlers contains the 4 SFTP server request handlers.
type Handlers struct {
	FileGet  FileReader
	FilePut  FileWriter
	FileCmd  FileCmder
	FileInfo FileInfoer
}

// RequestServer abstracts the sftp protocol with an http request-like protocol
type RequestServer struct {
	serverConn
	Handlers        Handlers
	pktMgr          packetManager
	openRequests    map[string]Request
	openRequestLock sync.RWMutex
	handleCount     int
}

// NewRequestServer creates/allocates/returns new RequestServer.
// Normally there there will be one server per user-session.
func NewRequestServer(rwc io.ReadWriteCloser, h Handlers) *RequestServer {
	svrConn := serverConn{
		conn: conn{
			Reader:      rwc,
			WriteCloser: rwc,
		},
	}
	return &RequestServer{
		serverConn:   svrConn,
		Handlers:     h,
		pktMgr:       newPktMgr(&svrConn),
		openRequests: make(map[string]Request),
	}
}

func (rs *RequestServer) nextRequest(r Request) string {
	rs.openRequestLock.Lock()
	defer rs.openRequestLock.Unlock()
	rs.handleCount++
	handle := strconv.Itoa(rs.handleCount)
	rs.openRequests[handle] = r
	return handle
}

func (rs *RequestServer) getRequest(handle string) (Request, bool) {
	rs.openRequestLock.RLock()
	defer rs.openRequestLock.RUnlock()
	r, ok := rs.openRequests[handle]
	return r, ok
}

func (rs *RequestServer) closeRequest(handle string) {
	rs.openRequestLock.Lock()
	defer rs.openRequestLock.Unlock()
	if r, ok := rs.openRequests[handle]; ok {
		r.close()
		delete(rs.openRequests, handle)
	}
}

// Close the read/write/closer to trigger exiting the main server loop
func (rs *RequestServer) Close() error { return rs.conn.Close() }

// Serve requests for user session
func (rs *RequestServer) Serve() error {
	var wg sync.WaitGroup
	wg.Add(1)
	workerFunc := func(ch requestChan) {
		wg.Add(1)
		defer wg.Done()
		if err := rs.packetWorker(ch); err != nil {
			rs.conn.Close() // shuts down recvPacket
		}
	}
	pktChan := rs.pktMgr.workerChan(workerFunc)

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
			debug("makePacket err: %v", err)
			rs.conn.Close() // shuts down recvPacket
			break
		}

		pktChan <- pkt
	}
	wg.Done()

	close(pktChan) // shuts down sftpServerWorkers
	wg.Wait()      // wait for all workers to exit

	return err
}

func (rs *RequestServer) packetWorker(pktChan chan requestPacket) error {
	for pkt := range pktChan {
		var rpkt responsePacket
		switch pkt := pkt.(type) {
		case *sshFxInitPacket:
			rpkt = sshFxVersionPacket{sftpProtocolVersion, nil}
		case *sshFxpClosePacket:
			handle := pkt.getHandle()
			rs.closeRequest(handle)
			rpkt = statusFromError(pkt, nil)
		case *sshFxpRealpathPacket:
			rpkt = cleanPath(pkt)
		case isOpener:
			handle := rs.nextRequest(requestFromPacket(pkt))
			rpkt = sshFxpHandlePacket{pkt.id(), handle}
		case *sshFxpFstatPacket:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt, syscall.EBADF)
			} else {
				request = requestFromPacket(
					&sshFxpStatPacket{ID: pkt.id(), Path: request.Filepath})
				rpkt = rs.handle(request, pkt)
			}
		case *sshFxpFsetstatPacket:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			if !ok {
				rpkt = statusFromError(pkt, syscall.EBADF)
			} else {
				request = requestFromPacket(
					&sshFxpSetstatPacket{ID: pkt.id(), Path: request.Filepath,
						Flags: pkt.Flags, Attrs: pkt.Attrs,
					})
				rpkt = rs.handle(request, pkt)
			}
		case hasHandle:
			handle := pkt.getHandle()
			request, ok := rs.getRequest(handle)
			request.update(pkt)
			if !ok {
				rpkt = statusFromError(pkt, syscall.EBADF)
			} else {
				rpkt = rs.handle(request, pkt)
			}
		case hasPath:
			request := requestFromPacket(pkt)
			rpkt = rs.handle(request, pkt)
		default:
			return errors.Errorf("unexpected packet type %T", pkt)
		}

		err := rs.sendPacket(rpkt)
		if err != nil {
			return err
		}
	}
	return nil
}

func cleanPath(pkt *sshFxpRealpathPacket) responsePacket {
	path := pkt.getPath()
	if !filepath.IsAbs(path) {
		path = "/" + path
	} // all paths are absolute

	cleaned_path := filepath.Clean(path)
	return &sshFxpNamePacket{
		ID: pkt.id(),
		NameAttrs: []sshFxpNameAttr{{
			Name:     cleaned_path,
			LongName: cleaned_path,
			Attrs:    emptyFileStat,
		}},
	}
}

func (rs *RequestServer) handle(request Request, pkt requestPacket) responsePacket {
	// fmt.Println("Request Method: ", request.Method)
	rpkt, err := request.handle(rs.Handlers)
	if err != nil {
		err = errorAdapter(err)
		rpkt = statusFromError(pkt, err)
	}
	return rpkt
}

// Wrap underlying connection methods to use packetManager
func (rs *RequestServer) sendPacket(m encoding.BinaryMarshaler) error {
	if pkt, ok := m.(responsePacket); ok {
		rs.pktMgr.readyPacket(pkt)
	} else {
		return errors.Errorf("unexpected packet type %T", m)
	}
	return nil
}

func (rs *RequestServer) sendError(p ider, err error) error {
	return rs.sendPacket(statusFromError(p, err))
}

// os.ErrNotExist should convert to ssh_FX_NO_SUCH_FILE, but is not recognized
// by statusFromError. So we convert to syscall.ENOENT which it does.
func errorAdapter(err error) error {
	if err == os.ErrNotExist {
		return syscall.ENOENT
	}
	return err
}
