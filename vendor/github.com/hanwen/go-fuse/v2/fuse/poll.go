package fuse

// Go 1.9 introduces polling for file I/O. The implementation causes
// the runtime's epoll to take up the last GOMAXPROCS slot, and if
// that happens, we won't have any threads left to service FUSE's
// _OP_POLL request. Prevent this by forcing _OP_POLL to happen, so we
// can say ENOSYS and prevent further _OP_POLL requests.
const pollHackName = ".go-fuse-epoll-hack"
const pollHackInode = ^uint64(0)

func doPollHackLookup(ms *Server, req *request) {
	attr := Attr{
		Ino:   pollHackInode,
		Mode:  S_IFREG | 0644,
		Nlink: 1,
	}
	switch req.inHeader.Opcode {
	case _OP_LOOKUP:
		out := (*EntryOut)(req.outData())
		*out = EntryOut{
			NodeId: pollHackInode,
			Attr:   attr,
		}
		req.status = OK
	case _OP_OPEN:
		out := (*OpenOut)(req.outData())
		*out = OpenOut{
			Fh: pollHackInode,
		}
		req.status = OK
	case _OP_GETATTR, _OP_SETATTR:
		out := (*AttrOut)(req.outData())
		out.Attr = attr
		req.status = OK
	case _OP_GETXATTR:
		// Kernel will try to read acl xattrs. Pretend we don't have any.
		req.status = ENODATA
	case _OP_POLL:
		req.status = ENOSYS

	case _OP_ACCESS, _OP_FLUSH, _OP_RELEASE:
		// Avoid upsetting the OSX mount process.
		req.status = OK
	default:
		// We want to avoid switching off features through our
		// poll hack, so don't use ENOSYS. It would be nice if
		// we could transmit no error code at all, but for
		// some opcodes, we'd have to invent credible data to
		// return as well.
		req.status = ERANGE
	}
}
