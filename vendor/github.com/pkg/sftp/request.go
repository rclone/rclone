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

// Request contains the data and state for the incoming service request.
type Request struct {
	// Get, Put, Setstat, Stat, Rename, Remove
	// Rmdir, Mkdir, List, Readlink, Symlink
	Method   string
	Filepath string
	Flags    uint32
	Attrs    []byte // convert to sub-struct
	Target   string // for renames and sym-links
	// packet data
	pkt_id  uint32
	packets chan packet_data
	// reader/writer/readdir from handlers
	stateLock *sync.RWMutex
	state     *state
}

type state struct {
	writerAt io.WriterAt
	readerAt io.ReaderAt
	endofdir bool // need to track when to send EOF for readdir
}

type packet_data struct {
	id     uint32
	data   []byte
	length uint32
	offset int64
}

// New Request initialized based on packet data
func requestFromPacket(pkt hasPath) Request {
	method := requestMethod(pkt)
	request := NewRequest(method, pkt.getPath())
	request.pkt_id = pkt.id()
	switch p := pkt.(type) {
	case *sshFxpSetstatPacket:
		request.Flags = p.Flags
		request.Attrs = p.Attrs.([]byte)
	case *sshFxpRenamePacket:
		request.Target = filepath.Clean(p.Newpath)
	case *sshFxpSymlinkPacket:
		request.Target = filepath.Clean(p.Linkpath)
	}
	return request
}

// NewRequest creates a new Request object.
func NewRequest(method, path string) Request {
	request := Request{Method: method, Filepath: filepath.Clean(path)}
	request.packets = make(chan packet_data, sftpServerWorkerCount)
	request.state = &state{}
	request.stateLock = &sync.RWMutex{}
	return request
}

// manage state
func (r Request) setState(s interface{}) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()
	switch s := s.(type) {
	case io.WriterAt:
		r.state.writerAt = s
	case io.ReaderAt:
		r.state.readerAt = s
	case bool:
		r.state.endofdir = s
	}
}

func (r Request) getWriter() io.WriterAt {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.writerAt
}

func (r Request) getReader() io.ReaderAt {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.readerAt
}

func (r Request) getEOD() bool {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state.endofdir
}

// Close reader/writer if possible
func (r Request) close() {
	rd := r.getReader()
	if c, ok := rd.(io.Closer); ok {
		c.Close()
	}
	wt := r.getWriter()
	if c, ok := wt.(io.Closer); ok {
		c.Close()
	}
}

// push packet_data into fifo
func (r Request) pushPacket(pd packet_data) {
	r.packets <- pd
}

// pop packet_data into fifo
func (r *Request) popPacket() packet_data {
	return <-r.packets
}

// called from worker to handle packet/request
func (r Request) handle(handlers Handlers) (responsePacket, error) {
	var err error
	var rpkt responsePacket
	switch r.Method {
	case "Get":
		rpkt, err = fileget(handlers.FileGet, r)
	case "Put": // add "Append" to this to handle append only file writes
		rpkt, err = fileput(handlers.FilePut, r)
	case "Setstat", "Rename", "Rmdir", "Mkdir", "Symlink", "Remove":
		rpkt, err = filecmd(handlers.FileCmd, r)
	case "List", "Stat", "Readlink":
		rpkt, err = fileinfo(handlers.FileInfo, r)
	default:
		return rpkt, errors.Errorf("unexpected method: %s", r.Method)
	}
	return rpkt, err
}

// wrap FileReader handler
func fileget(h FileReader, r Request) (responsePacket, error) {
	var err error
	reader := r.getReader()
	if reader == nil {
		reader, err = h.Fileread(r)
		if err != nil {
			return nil, err
		}
		r.setState(reader)
	}

	pd := r.popPacket()
	data := make([]byte, clamp(pd.length, maxTxPacket))
	n, err := reader.ReadAt(data, pd.offset)
	if err != nil && (err != io.EOF || n == 0) {
		return nil, err
	}
	return &sshFxpDataPacket{
		ID:     pd.id,
		Length: uint32(n),
		Data:   data[:n],
	}, nil
}

// wrap FileWriter handler
func fileput(h FileWriter, r Request) (responsePacket, error) {
	var err error
	writer := r.getWriter()
	if writer == nil {
		writer, err = h.Filewrite(r)
		if err != nil {
			return nil, err
		}
		r.setState(writer)
	}

	pd := r.popPacket()
	_, err = writer.WriteAt(pd.data, pd.offset)
	if err != nil {
		return nil, err
	}
	return &sshFxpStatusPacket{
		ID: pd.id,
		StatusError: StatusError{
			Code: ssh_FX_OK,
		}}, nil
}

// wrap FileCmder handler
func filecmd(h FileCmder, r Request) (responsePacket, error) {
	err := h.Filecmd(r)
	if err != nil {
		return nil, err
	}
	return &sshFxpStatusPacket{
		ID: r.pkt_id,
		StatusError: StatusError{
			Code: ssh_FX_OK,
		}}, nil
}

// wrap FileInfoer handler
func fileinfo(h FileInfoer, r Request) (responsePacket, error) {
	if r.getEOD() {
		return nil, io.EOF
	}
	finfo, err := h.Fileinfo(r)
	if err != nil {
		return nil, err
	}

	switch r.Method {
	case "List":
		pd := r.popPacket()
		dirname := path.Base(r.Filepath)
		ret := &sshFxpNamePacket{ID: pd.id}
		for _, fi := range finfo {
			ret.NameAttrs = append(ret.NameAttrs, sshFxpNameAttr{
				Name:     fi.Name(),
				LongName: runLs(dirname, fi),
				Attrs:    []interface{}{fi},
			})
		}
		r.setState(true)
		return ret, nil
	case "Stat":
		if len(finfo) == 0 {
			err = &os.PathError{Op: "stat", Path: r.Filepath,
				Err: syscall.ENOENT}
			return nil, err
		}
		return &sshFxpStatResponse{
			ID:   r.pkt_id,
			info: finfo[0],
		}, nil
	case "Readlink":
		if len(finfo) == 0 {
			err = &os.PathError{Op: "readlink", Path: r.Filepath,
				Err: syscall.ENOENT}
			return nil, err
		}
		filename := finfo[0].Name()
		return &sshFxpNamePacket{
			ID: r.pkt_id,
			NameAttrs: []sshFxpNameAttr{{
				Name:     filename,
				LongName: filename,
				Attrs:    emptyFileStat,
			}},
		}, nil
	}
	return nil, err
}

// file data for additional read/write packets
func (r *Request) update(p hasHandle) error {
	pd := packet_data{id: p.id()}
	switch p := p.(type) {
	case *sshFxpReadPacket:
		r.Method = "Get"
		pd.length = p.Len
		pd.offset = int64(p.Offset)
	case *sshFxpWritePacket:
		r.Method = "Put"
		pd.data = p.Data
		pd.length = p.Length
		pd.offset = int64(p.Offset)
	case *sshFxpReaddirPacket:
		r.Method = "List"
	default:
		return errors.Errorf("unexpected packet type %T", p)
	}
	r.pushPacket(pd)
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
