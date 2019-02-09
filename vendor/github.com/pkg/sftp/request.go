package sftp

import (
	"context"
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
	handle   string
	// reader/writer/readdir from handlers
	state state
	// context lasts duration of request
	ctx       context.Context
	cancelCtx context.CancelFunc
}

type state struct {
	*sync.RWMutex
	writerAt io.WriterAt
	readerAt io.ReaderAt
	listerAt ListerAt
	lsoffset int64
}

// New Request initialized based on packet data
func requestFromPacket(ctx context.Context, pkt hasPath) *Request {
	method := requestMethod(pkt)
	request := NewRequest(method, pkt.getPath())
	request.ctx, request.cancelCtx = context.WithCancel(ctx)

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
	return &Request{Method: method, Filepath: cleanPath(path),
		state: state{RWMutex: new(sync.RWMutex)}}
}

// shallow copy of existing request
func (r *Request) copy() *Request {
	r.state.Lock()
	defer r.state.Unlock()
	r2 := new(Request)
	*r2 = *r
	return r2
}

// Context returns the request's context. To change the context,
// use WithContext.
//
// The returned context is always non-nil; it defaults to the
// background context.
//
// For incoming server requests, the context is canceled when the
// request is complete or the client's connection closes.
func (r *Request) Context() context.Context {
	if r.ctx != nil {
		return r.ctx
	}
	return context.Background()
}

// WithContext returns a copy of r with its context changed to ctx.
// The provided ctx must be non-nil.
func (r *Request) WithContext(ctx context.Context) *Request {
	if ctx == nil {
		panic("nil context")
	}
	r2 := r.copy()
	r2.ctx = ctx
	r2.cancelCtx = nil
	return r2
}

// Returns current offset for file list
func (r *Request) lsNext() int64 {
	r.state.RLock()
	defer r.state.RUnlock()
	return r.state.lsoffset
}

// Increases next offset
func (r *Request) lsInc(offset int64) {
	r.state.Lock()
	defer r.state.Unlock()
	r.state.lsoffset = r.state.lsoffset + offset
}

// manage file read/write state
func (r *Request) setListerState(la ListerAt) {
	r.state.Lock()
	defer r.state.Unlock()
	r.state.listerAt = la
}

func (r *Request) getLister() ListerAt {
	r.state.RLock()
	defer r.state.RUnlock()
	return r.state.listerAt
}

// Close reader/writer if possible
func (r *Request) close() error {
	defer func() {
		if r.cancelCtx != nil {
			r.cancelCtx()
		}
	}()
	r.state.RLock()
	rd := r.state.readerAt
	r.state.RUnlock()
	if c, ok := rd.(io.Closer); ok {
		return c.Close()
	}
	r.state.RLock()
	wt := r.state.writerAt
	r.state.RUnlock()
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
	case "Put":
		return fileput(handlers.FilePut, r, pkt)
	case "Setstat", "Rename", "Rmdir", "Mkdir", "Symlink", "Remove":
		return filecmd(handlers.FileCmd, r, pkt)
	case "List":
		return filelist(handlers.FileList, r, pkt)
	case "Stat", "Readlink":
		return filestat(handlers.FileList, r, pkt)
	default:
		return statusFromError(pkt,
			errors.Errorf("unexpected method: %s", r.Method))
	}
}

// Additional initialization for Open packets
func (r *Request) open(h Handlers, pkt requestPacket) responsePacket {
	flags := r.Pflags()
	var err error
	switch {
	case flags.Write, flags.Append, flags.Creat, flags.Trunc:
		r.Method = "Put"
		r.state.writerAt, err = h.FilePut.Filewrite(r)
	case flags.Read:
		r.Method = "Get"
		r.state.readerAt, err = h.FileGet.Fileread(r)
	default:
		return statusFromError(pkt, errors.New("bad file flags"))
	}
	if err != nil {
		return statusFromError(pkt, err)
	}
	return &sshFxpHandlePacket{ID: pkt.id(), Handle: r.handle}
}
func (r *Request) opendir(h Handlers, pkt requestPacket) responsePacket {
	var err error
	r.Method = "List"
	r.state.listerAt, err = h.FileList.Filelist(r)
	if err != nil {
		switch err.(type) {
		case syscall.Errno:
			err = &os.PathError{Path: r.Filepath, Err: err}
		}
		return statusFromError(pkt, err)
	}
	return &sshFxpHandlePacket{ID: pkt.id(), Handle: r.handle}
}

// wrap FileReader handler
func fileget(h FileReader, r *Request, pkt requestPacket) responsePacket {
	//fmt.Println("fileget", r)
	r.state.RLock()
	reader := r.state.readerAt
	r.state.RUnlock()
	if reader == nil {
		return statusFromError(pkt, errors.New("unexpected read packet"))
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
	//fmt.Println("fileput", r)
	r.state.RLock()
	writer := r.state.writerAt
	r.state.RUnlock()
	if writer == nil {
		return statusFromError(pkt, errors.New("unexpected write packet"))
	}

	data, offset, _ := packetData(pkt)
	_, err := writer.WriteAt(data, offset)
	return statusFromError(pkt, err)
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

// wrap FileCmder handler
func filecmd(h FileCmder, r *Request, pkt requestPacket) responsePacket {

	switch p := pkt.(type) {
	case *sshFxpFsetstatPacket:
		r.Flags = p.Flags
		r.Attrs = p.Attrs.([]byte)
	}
	err := h.Filecmd(r)
	return statusFromError(pkt, err)
}

// wrap FileLister handler
func filelist(h FileLister, r *Request, pkt requestPacket) responsePacket {
	var err error
	lister := r.getLister()
	if lister == nil {
		return statusFromError(pkt, errors.New("unexpected dir packet"))
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
		if err == io.EOF && n == 0 {
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
	default:
		err = errors.Errorf("unexpected method: %s", r.Method)
		return statusFromError(pkt, err)
	}
}

func filestat(h FileLister, r *Request, pkt requestPacket) responsePacket {
	lister, err := h.Filelist(r)
	if err != nil {
		return statusFromError(pkt, err)
	}
	finfo := make([]os.FileInfo, 1)
	n, err := lister.ListAt(finfo, 0)
	finfo = finfo[:n] // avoid need for nil tests below

	switch r.Method {
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
	case *sshFxpReadPacket, *sshFxpWritePacket, *sshFxpOpenPacket:
		// set in open() above
	case *sshFxpOpendirPacket, *sshFxpReaddirPacket:
		// set in opendir() above
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
