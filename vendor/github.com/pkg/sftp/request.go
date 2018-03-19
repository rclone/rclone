package sftp

import (
	"io"
	"os"
	"path"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/pkg/errors"
)

// MaxFilelist is the max number of files to return in a readdir batch.
var MaxFilelist int64 = 100

// Request contains the data and state for the incoming service request.
type Request struct {
	// Get, Put, Setstat, Stat, Rename, Remove
	// Rmdir, Mkdir, List, Readlink, Symlink
	Method   string
	Filepath string
	Flags    uint32
	Attrs    []byte // convert to sub-struct
	Target   string // for renames and sym-links
	// reader/writer/readdir from handlers
	stateLock sync.RWMutex
	state     state
}

type state struct {
	writerAt io.WriterAt
	readerAt io.ReaderAt
	listerAt ListerAt
	lsoffset int64
}

// New Request initialized based on packet data
func requestFromPacket(pkt hasPath) *Request {
	method := requestMethod(pkt)
	request := NewRequest(method, pkt.getPath())
	switch p := pkt.(type) {
	case *sshFxpOpenPacket:
		request.Flags = p.Pflags
	case *sshFxpSetstatPacket:
		request.Flags = p.Flags
		request.Attrs = p.Attrs.([]byte)
	case *sshFxpRenamePacket:
		request.Target = cleanPath(p.Newpath)
	case *sshFxpSymlinkPacket:
		request.Target = cleanPath(p.Linkpath)
	}
	return request
}

// NewRequest creates a new Request object.
func NewRequest(method, path string) *Request {
	return &Request{Method: method, Filepath: cleanPath(path)}
}

// Returns current offset for file list
func (r *Request) lsNext() int64 {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.lsoffset
}

// Increases next offset
func (r *Request) lsInc(offset int64) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	r.state.lsoffset = r.state.lsoffset + offset
}

// manage file read/write state
func (r *Request) setWriterState(wa io.WriterAt) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	r.state.writerAt = wa
}
func (r *Request) setReaderState(ra io.ReaderAt) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	r.state.readerAt = ra
}
func (r *Request) setListerState(la ListerAt) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	r.state.listerAt = la
}

func (r *Request) getWriter() io.WriterAt {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.writerAt
}

func (r *Request) getReader() io.ReaderAt {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.readerAt
}

func (r *Request) getLister() ListerAt {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.listerAt
}

// Close reader/writer if possible
func (r *Request) close() error {
	rd := r.getReader()
	if c, ok := rd.(io.Closer); ok {
		return c.Close()
	}
	wt := r.getWriter()
	if c, ok := wt.(io.Closer); ok {
		return c.Close()
	}
	return nil
}

// called from worker to handle packet/request
func (r *Request) call(handlers Handlers, pkt requestPacket) responsePacket {
	switch r.Method {
	case "Get":
		return fileget(handlers.FileGet, r, pkt)
	case "Put", "Open":
		return fileput(handlers.FilePut, r, pkt)
	case "Setstat", "Rename", "Rmdir", "Mkdir", "Symlink", "Remove":
		return filecmd(handlers.FileCmd, r, pkt)
	case "List", "Stat", "Readlink":
		return filelist(handlers.FileList, r, pkt)
	default:
		return statusFromError(pkt,
			errors.Errorf("unexpected method: %s", r.Method))
	}
}

// file data for additional read/write packets
func packetData(p requestPacket) (data []byte, offset int64, length uint32) {
	switch p := p.(type) {
	case *sshFxpReadPacket:
		length = p.Len
		offset = int64(p.Offset)
	case *sshFxpWritePacket:
		data = p.Data
		length = p.Length
		offset = int64(p.Offset)
	}
	return
}

// wrap FileReader handler
func fileget(h FileReader, r *Request, pkt requestPacket) responsePacket {
	var err error
	reader := r.getReader()
	if reader == nil {
		reader, err = h.Fileread(r)
		if err != nil {
			return statusFromError(pkt, err)
		}
		r.setReaderState(reader)
	}

	_, offset, length := packetData(pkt)
	data := make([]byte, clamp(length, maxTxPacket))
	n, err := reader.ReadAt(data, offset)
	// only return EOF erro if no data left to read
	if err != nil && (err != io.EOF || n == 0) {
		return statusFromError(pkt, err)
	}
	return &sshFxpDataPacket{
		ID:     pkt.id(),
		Length: uint32(n),
		Data:   data[:n],
	}
}

// wrap FileWriter handler
func fileput(h FileWriter, r *Request, pkt requestPacket) responsePacket {
	var err error
	writer := r.getWriter()
	if writer == nil {
		writer, err = h.Filewrite(r)
		if err != nil {
			return statusFromError(pkt, err)
		}
		r.setWriterState(writer)
	}

	data, offset, _ := packetData(pkt)
	_, err = writer.WriteAt(data, offset)
	return statusFromError(pkt, err)
}

// wrap FileCmder handler
func filecmd(h FileCmder, r *Request, pkt requestPacket) responsePacket {
	err := h.Filecmd(r)
	return statusFromError(pkt, err)
}

// wrap FileLister handler
func filelist(h FileLister, r *Request, pkt requestPacket) responsePacket {
	var err error
	lister := r.getLister()
	if lister == nil {
		lister, err = h.Filelist(r)
		if err != nil {
			return statusFromError(pkt, err)
		}
		r.setListerState(lister)
	}

	offset := r.lsNext()
	finfo := make([]os.FileInfo, MaxFilelist)
	n, err := lister.ListAt(finfo, offset)
	r.lsInc(int64(n))
	// ignore EOF as we only return it when there are no results
	finfo = finfo[:n] // avoid need for nil tests below

	switch r.Method {
	case "List":
		if err != nil && err != io.EOF {
			return statusFromError(pkt, err)
		}
		if n == 0 {
			return statusFromError(pkt, io.EOF)
		}
		dirname := filepath.ToSlash(path.Base(r.Filepath))
		ret := &sshFxpNamePacket{ID: pkt.id()}

		for _, fi := range finfo {
			ret.NameAttrs = append(ret.NameAttrs, sshFxpNameAttr{
				Name:     fi.Name(),
				LongName: runLs(dirname, fi),
				Attrs:    []interface{}{fi},
			})
		}
		return ret
	case "Stat":
		if err != nil && err != io.EOF {
			return statusFromError(pkt, err)
		}
		if n == 0 {
			err = &os.PathError{Op: "stat", Path: r.Filepath,
				Err: syscall.ENOENT}
			return statusFromError(pkt, err)
		}
		return &sshFxpStatResponse{
			ID:   pkt.id(),
			info: finfo[0],
		}
	case "Readlink":
		if err != nil && err != io.EOF {
			return statusFromError(pkt, err)
		}
		if n == 0 {
			err = &os.PathError{Op: "readlink", Path: r.Filepath,
				Err: syscall.ENOENT}
			return statusFromError(pkt, err)
		}
		filename := finfo[0].Name()
		return &sshFxpNamePacket{
			ID: pkt.id(),
			NameAttrs: []sshFxpNameAttr{{
				Name:     filename,
				LongName: filename,
				Attrs:    emptyFileStat,
			}},
		}
	default:
		err = errors.Errorf("unexpected method: %s", r.Method)
		return statusFromError(pkt, err)
	}
}

// init attributes of request object from packet data
func requestMethod(p requestPacket) (method string) {
	switch p.(type) {
	case *sshFxpReadPacket:
		method = "Get"
	case *sshFxpWritePacket:
		method = "Put"
	case *sshFxpReaddirPacket:
		method = "List"
	case *sshFxpOpenPacket, *sshFxpOpendirPacket:
		method = "Open"
	case *sshFxpSetstatPacket, *sshFxpFsetstatPacket:
		method = "Setstat"
	case *sshFxpRenamePacket:
		method = "Rename"
	case *sshFxpSymlinkPacket:
		method = "Symlink"
	case *sshFxpRemovePacket:
		method = "Remove"
	case *sshFxpStatPacket, *sshFxpLstatPacket, *sshFxpFstatPacket:
		method = "Stat"
	case *sshFxpRmdirPacket:
		method = "Rmdir"
	case *sshFxpReadlinkPacket:
		method = "Readlink"
	case *sshFxpMkdirPacket:
		method = "Mkdir"
	}
	return method
}
