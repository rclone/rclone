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
	stateLock *sync.RWMutex
	state     *state
}

type state struct {
	writerAt io.WriterAt
	readerAt io.ReaderAt
	listerAt ListerAt
	lsoffset int64
}

type packet_data struct {
	_id    uint32
	data   []byte
	length uint32
	offset int64
}

func (pd packet_data) id() uint32 {
	return pd._id
}

// New Request initialized based on packet data
func requestFromPacket(pkt hasPath) *Request {
	method := requestMethod(pkt)
	request := NewRequest(method, pkt.getPath())
	switch p := pkt.(type) {
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

func newRequest() *Request {
	return &Request{state: &state{}, stateLock: &sync.RWMutex{}}
}

// NewRequest creates a new Request object.
func NewRequest(method, path string) *Request {
	request := newRequest()
	request.Method = method
	request.Filepath = cleanPath(path)
	return request
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
func (r *Request) setFileState(s interface{}) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	switch s := s.(type) {
	case io.WriterAt:
		r.state.writerAt = s
	case io.ReaderAt:
		r.state.readerAt = s
	case ListerAt:
		r.state.listerAt = s
	case int64:
		r.state.lsoffset = s
	}
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
func (r *Request) close() {
	rd := r.getReader()
	if c, ok := rd.(io.Closer); ok {
		c.Close()
	}
	wt := r.getWriter()
	if c, ok := wt.(io.Closer); ok {
		c.Close()
	}
}

// called from worker to handle packet/request
func (r *Request) call(handlers Handlers, pkt requestPacket) responsePacket {
	pd := packetData(pkt)
	switch r.Method {
	case "Get":
		return fileget(handlers.FileGet, r, pd)
	case "Put": // add "Append" to this to handle append only file writes
		return fileput(handlers.FilePut, r, pd)
	case "Setstat", "Rename", "Rmdir", "Mkdir", "Symlink", "Remove":
		return filecmd(handlers.FileCmd, r, pd)
	case "List", "Stat", "Readlink":
		return filelist(handlers.FileList, r, pd)
	default:
		return statusFromError(pkt,
			errors.Errorf("unexpected method: %s", r.Method))
	}
}

// file data for additional read/write packets
func packetData(p requestPacket) packet_data {
	pd := packet_data{_id: p.id()}
	switch p := p.(type) {
	case *sshFxpReadPacket:
		pd.length = p.Len
		pd.offset = int64(p.Offset)
	case *sshFxpWritePacket:
		pd.data = p.Data
		pd.length = p.Length
		pd.offset = int64(p.Offset)
	}
	return pd
}

// wrap FileReader handler
func fileget(h FileReader, r *Request, pd packet_data) responsePacket {
	var err error
	reader := r.getReader()
	if reader == nil {
		reader, err = h.Fileread(r)
		if err != nil {
			return statusFromError(pd, err)
		}
		r.setFileState(reader)
	}

	data := make([]byte, clamp(pd.length, maxTxPacket))
	n, err := reader.ReadAt(data, pd.offset)
	// only return EOF erro if no data left to read
	if err != nil && (err != io.EOF || n == 0) {
		return statusFromError(pd, err)
	}
	return &sshFxpDataPacket{
		ID:     pd.id(),
		Length: uint32(n),
		Data:   data[:n],
	}
}

// wrap FileWriter handler
func fileput(h FileWriter, r *Request, pd packet_data) responsePacket {
	var err error
	writer := r.getWriter()
	if writer == nil {
		writer, err = h.Filewrite(r)
		if err != nil {
			return statusFromError(pd, err)
		}
		r.setFileState(writer)
	}

	_, err = writer.WriteAt(pd.data, pd.offset)
	if err != nil {
		return statusFromError(pd, err)
	}
	return &sshFxpStatusPacket{
		ID: pd.id(),
		StatusError: StatusError{
			Code: ssh_FX_OK,
		}}
}

// wrap FileCmder handler
func filecmd(h FileCmder, r *Request, pd packet_data) responsePacket {
	err := h.Filecmd(r)
	if err != nil {
		return statusFromError(pd, err)
	}
	return &sshFxpStatusPacket{
		ID: pd.id(),
		StatusError: StatusError{
			Code: ssh_FX_OK,
		}}
}

// wrap FileLister handler
func filelist(h FileLister, r *Request, pd packet_data) responsePacket {
	var err error
	lister := r.getLister()
	if lister == nil {
		lister, err = h.Filelist(r)
		if err != nil {
			return statusFromError(pd, err)
		}
		r.setFileState(lister)
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
			return statusFromError(pd, err)
		}
		if n == 0 {
			return statusFromError(pd, io.EOF)
		}
		dirname := filepath.ToSlash(path.Base(r.Filepath))
		ret := &sshFxpNamePacket{ID: pd.id()}

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
			return statusFromError(pd, err)
		}
		if n == 0 {
			err = &os.PathError{Op: "stat", Path: r.Filepath,
				Err: syscall.ENOENT}
			return statusFromError(pd, err)
		}
		return &sshFxpStatResponse{
			ID:   pd.id(),
			info: finfo[0],
		}
	case "Readlink":
		if err != nil && err != io.EOF {
			return statusFromError(pd, err)
		}
		if n == 0 {
			err = &os.PathError{Op: "readlink", Path: r.Filepath,
				Err: syscall.ENOENT}
			return statusFromError(pd, err)
		}
		filename := finfo[0].Name()
		return &sshFxpNamePacket{
			ID: pd.id(),
			NameAttrs: []sshFxpNameAttr{{
				Name:     filename,
				LongName: filename,
				Attrs:    emptyFileStat,
			}},
		}
	default:
		err = errors.Errorf("unexpected method: %s", r.Method)
		return statusFromError(pd, err)
	}
}

// file data for additional read/write packets
func (r *Request) updateMethod(p hasHandle) error {
	switch p := p.(type) {
	case *sshFxpReadPacket:
		r.Method = "Get"
	case *sshFxpWritePacket:
		r.Method = "Put"
	case *sshFxpReaddirPacket:
		r.Method = "List"
	default:
		return errors.Errorf("unexpected packet type %T", p)
	}
	return nil
}

// init attributes of request object from packet data
func requestMethod(p hasPath) (method string) {
	switch p.(type) {
	case *sshFxpOpenPacket, *sshFxpOpendirPacket:
		method = "Open"
	case *sshFxpSetstatPacket:
		method = "Setstat"
	case *sshFxpRenamePacket:
		method = "Rename"
	case *sshFxpSymlinkPacket:
		method = "Symlink"
	case *sshFxpRemovePacket:
		method = "Remove"
	case *sshFxpStatPacket, *sshFxpLstatPacket:
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
