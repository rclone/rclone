package sftp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"syscall"
)

// MaxFilelist is the max number of files to return in a readdir batch.
var MaxFilelist int64 = 100

// state encapsulates the reader/writer/readdir from handlers.
type state struct {
	mu sync.RWMutex

	writerAt         io.WriterAt
	readerAt         io.ReaderAt
	writerAtReaderAt WriterAtReaderAt
	listerAt         ListerAt
	lsoffset         int64
}

// copy returns a shallow copy the state.
// This is broken out to specific fields,
// because we have to copy around the mutex in state.
func (s *state) copy() state {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return state{
		writerAt:         s.writerAt,
		readerAt:         s.readerAt,
		writerAtReaderAt: s.writerAtReaderAt,
		listerAt:         s.listerAt,
		lsoffset:         s.lsoffset,
	}
}

func (s *state) setReaderAt(rd io.ReaderAt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.readerAt = rd
}

func (s *state) getReaderAt() io.ReaderAt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readerAt
}

func (s *state) setWriterAt(rd io.WriterAt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writerAt = rd
}

func (s *state) getWriterAt() io.WriterAt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.writerAt
}

func (s *state) setWriterAtReaderAt(rw WriterAtReaderAt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.writerAtReaderAt = rw
}

func (s *state) getWriterAtReaderAt() WriterAtReaderAt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.writerAtReaderAt
}

func (s *state) getAllReaderWriters() (io.ReaderAt, io.WriterAt, WriterAtReaderAt) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.readerAt, s.writerAt, s.writerAtReaderAt
}

// Returns current offset for file list
func (s *state) lsNext() int64 {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lsoffset
}

// Increases next offset
func (s *state) lsInc(offset int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.lsoffset += offset
}

// manage file read/write state
func (s *state) setListerAt(la ListerAt) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.listerAt = la
}

func (s *state) getListerAt() ListerAt {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.listerAt
}

// Request contains the data and state for the incoming service request.
type Request struct {
	// Get, Put, Setstat, Stat, Rename, Remove
	// Rmdir, Mkdir, List, Readlink, Link, Symlink
	Method   string
	Filepath string
	Flags    uint32
	Attrs    []byte // convert to sub-struct
	Target   string // for renames and sym-links
	handle   string

	// reader/writer/readdir from handlers
	state

	// context lasts duration of request
	ctx       context.Context
	cancelCtx context.CancelFunc
}

// NewRequest creates a new Request object.
func NewRequest(method, path string) *Request {
	return &Request{
		Method:   method,
		Filepath: cleanPath(path),
	}
}

// copy returns a shallow copy of existing request.
// This is broken out to specific fields,
// because we have to copy around the mutex in state.
func (r *Request) copy() *Request {
	return &Request{
		Method:   r.Method,
		Filepath: r.Filepath,
		Flags:    r.Flags,
		Attrs:    r.Attrs,
		Target:   r.Target,
		handle:   r.handle,

		state: r.state.copy(),

		ctx:       r.ctx,
		cancelCtx: r.cancelCtx,
	}
}

// New Request initialized based on packet data
func requestFromPacket(ctx context.Context, pkt hasPath, baseDir string) *Request {
	request := &Request{
		Method:   requestMethod(pkt),
		Filepath: cleanPathWithBase(baseDir, pkt.getPath()),
	}
	request.ctx, request.cancelCtx = context.WithCancel(ctx)

	switch p := pkt.(type) {
	case *sshFxpOpenPacket:
		request.Flags = p.Pflags
	case *sshFxpSetstatPacket:
		request.Flags = p.Flags
		request.Attrs = p.Attrs.([]byte)
	case *sshFxpRenamePacket:
		request.Target = cleanPathWithBase(baseDir, p.Newpath)
	case *sshFxpSymlinkPacket:
		// NOTE: given a POSIX compliant signature: symlink(target, linkpath string)
		// this makes Request.Target the linkpath, and Request.Filepath the target.
		request.Target = cleanPathWithBase(baseDir, p.Linkpath)
		request.Filepath = p.Targetpath
	case *sshFxpExtendedPacketHardlink:
		request.Target = cleanPathWithBase(baseDir, p.Newpath)
	}
	return request
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

// Close reader/writer if possible
func (r *Request) close() error {
	defer func() {
		if r.cancelCtx != nil {
			r.cancelCtx()
		}
	}()

	rd, wr, rw := r.getAllReaderWriters()

	var err error

	// Close errors on a Writer are far more likely to be the important one.
	// As they can be information that there was a loss of data.
	if c, ok := wr.(io.Closer); ok {
		if err2 := c.Close(); err == nil {
			// update error if it is still nil
			err = err2
		}
	}

	if c, ok := rw.(io.Closer); ok {
		if err2 := c.Close(); err == nil {
			// update error if it is still nil
			err = err2

			r.setWriterAtReaderAt(nil)
		}
	}

	if c, ok := rd.(io.Closer); ok {
		if err2 := c.Close(); err == nil {
			// update error if it is still nil
			err = err2
		}
	}

	return err
}

// Notify transfer error if any
func (r *Request) transferError(err error) {
	if err == nil {
		return
	}

	rd, wr, rw := r.getAllReaderWriters()

	if t, ok := wr.(TransferError); ok {
		t.TransferError(err)
	}

	if t, ok := rw.(TransferError); ok {
		t.TransferError(err)
	}

	if t, ok := rd.(TransferError); ok {
		t.TransferError(err)
	}
}

// called from worker to handle packet/request
func (r *Request) call(handlers Handlers, pkt requestPacket, alloc *allocator, orderID uint32) responsePacket {
	switch r.Method {
	case "Get":
		return fileget(handlers.FileGet, r, pkt, alloc, orderID)
	case "Put":
		return fileput(handlers.FilePut, r, pkt, alloc, orderID)
	case "Open":
		return fileputget(handlers.FilePut, r, pkt, alloc, orderID)
	case "Setstat", "Rename", "Rmdir", "Mkdir", "Link", "Symlink", "Remove", "PosixRename", "StatVFS":
		return filecmd(handlers.FileCmd, r, pkt)
	case "List":
		return filelist(handlers.FileList, r, pkt)
	case "Stat", "Lstat":
		return filestat(handlers.FileList, r, pkt)
	case "Readlink":
		if readlinkFileLister, ok := handlers.FileList.(ReadlinkFileLister); ok {
			return readlink(readlinkFileLister, r, pkt)
		}
		return filestat(handlers.FileList, r, pkt)
	default:
		return statusFromError(pkt.id(), fmt.Errorf("unexpected method: %s", r.Method))
	}
}

// Additional initialization for Open packets
func (r *Request) open(h Handlers, pkt requestPacket) responsePacket {
	flags := r.Pflags()

	id := pkt.id()

	switch {
	case flags.Write, flags.Append, flags.Creat, flags.Trunc:
		if flags.Read {
			if openFileWriter, ok := h.FilePut.(OpenFileWriter); ok {
				r.Method = "Open"
				rw, err := openFileWriter.OpenFile(r)
				if err != nil {
					return statusFromError(id, err)
				}

				r.setWriterAtReaderAt(rw)

				return &sshFxpHandlePacket{
					ID:     id,
					Handle: r.handle,
				}
			}
		}

		r.Method = "Put"
		wr, err := h.FilePut.Filewrite(r)
		if err != nil {
			return statusFromError(id, err)
		}

		r.setWriterAt(wr)

	case flags.Read:
		r.Method = "Get"
		rd, err := h.FileGet.Fileread(r)
		if err != nil {
			return statusFromError(id, err)
		}

		r.setReaderAt(rd)

	default:
		return statusFromError(id, errors.New("bad file flags"))
	}

	return &sshFxpHandlePacket{
		ID:     id,
		Handle: r.handle,
	}
}

func (r *Request) opendir(h Handlers, pkt requestPacket) responsePacket {
	r.Method = "List"
	la, err := h.FileList.Filelist(r)
	if err != nil {
		return statusFromError(pkt.id(), wrapPathError(r.Filepath, err))
	}

	r.setListerAt(la)

	return &sshFxpHandlePacket{
		ID:     pkt.id(),
		Handle: r.handle,
	}
}

// wrap FileReader handler
func fileget(h FileReader, r *Request, pkt requestPacket, alloc *allocator, orderID uint32) responsePacket {
	rd := r.getReaderAt()
	if rd == nil {
		return statusFromError(pkt.id(), errors.New("unexpected read packet"))
	}

	data, offset, _ := packetData(pkt, alloc, orderID)

	n, err := rd.ReadAt(data, offset)
	// only return EOF error if no data left to read
	if err != nil && (err != io.EOF || n == 0) {
		return statusFromError(pkt.id(), err)
	}

	return &sshFxpDataPacket{
		ID:     pkt.id(),
		Length: uint32(n),
		Data:   data[:n],
	}
}

// wrap FileWriter handler
func fileput(h FileWriter, r *Request, pkt requestPacket, alloc *allocator, orderID uint32) responsePacket {
	wr := r.getWriterAt()
	if wr == nil {
		return statusFromError(pkt.id(), errors.New("unexpected write packet"))
	}

	data, offset, _ := packetData(pkt, alloc, orderID)

	_, err := wr.WriteAt(data, offset)
	return statusFromError(pkt.id(), err)
}

// wrap OpenFileWriter handler
func fileputget(h FileWriter, r *Request, pkt requestPacket, alloc *allocator, orderID uint32) responsePacket {
	rw := r.getWriterAtReaderAt()
	if rw == nil {
		return statusFromError(pkt.id(), errors.New("unexpected write and read packet"))
	}

	switch p := pkt.(type) {
	case *sshFxpReadPacket:
		data, offset := p.getDataSlice(alloc, orderID), int64(p.Offset)

		n, err := rw.ReadAt(data, offset)
		// only return EOF error if no data left to read
		if err != nil && (err != io.EOF || n == 0) {
			return statusFromError(pkt.id(), err)
		}

		return &sshFxpDataPacket{
			ID:     pkt.id(),
			Length: uint32(n),
			Data:   data[:n],
		}

	case *sshFxpWritePacket:
		data, offset := p.Data, int64(p.Offset)

		_, err := rw.WriteAt(data, offset)
		return statusFromError(pkt.id(), err)

	default:
		return statusFromError(pkt.id(), errors.New("unexpected packet type for read or write"))
	}
}

// file data for additional read/write packets
func packetData(p requestPacket, alloc *allocator, orderID uint32) (data []byte, offset int64, length uint32) {
	switch p := p.(type) {
	case *sshFxpReadPacket:
		return p.getDataSlice(alloc, orderID), int64(p.Offset), p.Len
	case *sshFxpWritePacket:
		return p.Data, int64(p.Offset), p.Length
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

	switch r.Method {
	case "PosixRename":
		if posixRenamer, ok := h.(PosixRenameFileCmder); ok {
			err := posixRenamer.PosixRename(r)
			return statusFromError(pkt.id(), err)
		}

		// PosixRenameFileCmder not implemented handle this request as a Rename
		r.Method = "Rename"
		err := h.Filecmd(r)
		return statusFromError(pkt.id(), err)

	case "StatVFS":
		if statVFSCmdr, ok := h.(StatVFSFileCmder); ok {
			stat, err := statVFSCmdr.StatVFS(r)
			if err != nil {
				return statusFromError(pkt.id(), err)
			}
			stat.ID = pkt.id()
			return stat
		}

		return statusFromError(pkt.id(), ErrSSHFxOpUnsupported)
	}

	err := h.Filecmd(r)
	return statusFromError(pkt.id(), err)
}

// wrap FileLister handler
func filelist(h FileLister, r *Request, pkt requestPacket) responsePacket {
	lister := r.getListerAt()
	if lister == nil {
		return statusFromError(pkt.id(), errors.New("unexpected dir packet"))
	}

	offset := r.lsNext()
	finfo := make([]os.FileInfo, MaxFilelist)
	n, err := lister.ListAt(finfo, offset)
	r.lsInc(int64(n))
	// ignore EOF as we only return it when there are no results
	finfo = finfo[:n] // avoid need for nil tests below

	switch r.Method {
	case "List":
		if err != nil && (err != io.EOF || n == 0) {
			return statusFromError(pkt.id(), err)
		}

		nameAttrs := make([]*sshFxpNameAttr, 0, len(finfo))

		// If the type conversion fails, we get untyped `nil`,
		// which is handled by not looking up any names.
		idLookup, _ := h.(NameLookupFileLister)

		for _, fi := range finfo {
			nameAttrs = append(nameAttrs, &sshFxpNameAttr{
				Name:     fi.Name(),
				LongName: runLs(idLookup, fi),
				Attrs:    []interface{}{fi},
			})
		}

		return &sshFxpNamePacket{
			ID:        pkt.id(),
			NameAttrs: nameAttrs,
		}

	default:
		err = fmt.Errorf("unexpected method: %s", r.Method)
		return statusFromError(pkt.id(), err)
	}
}

func filestat(h FileLister, r *Request, pkt requestPacket) responsePacket {
	var lister ListerAt
	var err error

	if r.Method == "Lstat" {
		if lstatFileLister, ok := h.(LstatFileLister); ok {
			lister, err = lstatFileLister.Lstat(r)
		} else {
			// LstatFileLister not implemented handle this request as a Stat
			r.Method = "Stat"
			lister, err = h.Filelist(r)
		}
	} else {
		lister, err = h.Filelist(r)
	}
	if err != nil {
		return statusFromError(pkt.id(), err)
	}
	finfo := make([]os.FileInfo, 1)
	n, err := lister.ListAt(finfo, 0)
	finfo = finfo[:n] // avoid need for nil tests below

	switch r.Method {
	case "Stat", "Lstat":
		if err != nil && err != io.EOF {
			return statusFromError(pkt.id(), err)
		}
		if n == 0 {
			err = &os.PathError{
				Op:   strings.ToLower(r.Method),
				Path: r.Filepath,
				Err:  syscall.ENOENT,
			}
			return statusFromError(pkt.id(), err)
		}
		return &sshFxpStatResponse{
			ID:   pkt.id(),
			info: finfo[0],
		}
	case "Readlink":
		if err != nil && err != io.EOF {
			return statusFromError(pkt.id(), err)
		}
		if n == 0 {
			err = &os.PathError{
				Op:   "readlink",
				Path: r.Filepath,
				Err:  syscall.ENOENT,
			}
			return statusFromError(pkt.id(), err)
		}
		filename := finfo[0].Name()
		return &sshFxpNamePacket{
			ID: pkt.id(),
			NameAttrs: []*sshFxpNameAttr{
				{
					Name:     filename,
					LongName: filename,
					Attrs:    emptyFileStat,
				},
			},
		}
	default:
		err = fmt.Errorf("unexpected method: %s", r.Method)
		return statusFromError(pkt.id(), err)
	}
}

func readlink(readlinkFileLister ReadlinkFileLister, r *Request, pkt requestPacket) responsePacket {
	resolved, err := readlinkFileLister.Readlink(r.Filepath)
	if err != nil {
		return statusFromError(pkt.id(), err)
	}
	return &sshFxpNamePacket{
		ID: pkt.id(),
		NameAttrs: []*sshFxpNameAttr{
			{
				Name:     resolved,
				LongName: resolved,
				Attrs:    emptyFileStat,
			},
		},
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
	case *sshFxpStatPacket, *sshFxpFstatPacket:
		method = "Stat"
	case *sshFxpLstatPacket:
		method = "Lstat"
	case *sshFxpRmdirPacket:
		method = "Rmdir"
	case *sshFxpReadlinkPacket:
		method = "Readlink"
	case *sshFxpMkdirPacket:
		method = "Mkdir"
	case *sshFxpExtendedPacketHardlink:
		method = "Link"
	}
	return method
}
