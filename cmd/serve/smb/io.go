package smb

import (
	"errors"
	"io"
)

// maxIOSize is the largest single READ/WRITE payload we accept. It matches the
// MaxReadSize/MaxWriteSize advertised in NEGOTIATE, so a client can't drive an
// arbitrarily large allocation off a wire-supplied length.
const maxIOSize = 0x00100000 // 1 MiB

// handleRead handles an SMB2 READ request ([MS-SMB2] 2.2.19).
func (c *conn) handleRead(h header, body []byte) (uint32, []byte) {
	if len(body) < 48 {
		return statusInvalidParameter, errorResponseBody()
	}
	length := le.Uint32(body[4:8])
	offset := le.Uint64(body[8:16])
	of := c.getHandle(body[16:32])
	if of == nil || of.handle == nil {
		return statusInvalidParameter, errorResponseBody()
	}
	// Reject reads larger than the advertised MaxReadSize rather than allocating
	// an attacker-chosen buffer (a 4 GiB Length would otherwise OOM/panic here).
	if length > maxIOSize {
		return statusInvalidParameter, errorResponseBody()
	}
	if length == 0 {
		return statusSuccess, readResponseBody(nil)
	}

	buf := make([]byte, length)
	n, err := of.handle.ReadAt(buf, int64(offset))
	if n <= 0 {
		// A read entirely at or after EOF must be answered with
		// STATUS_END_OF_FILE, which the client turns into io.EOF.
		if err == nil || errors.Is(err, io.EOF) {
			return statusEndOfFile, errorResponseBody()
		}
		return mapVFSError(err), errorResponseBody()
	}
	return statusSuccess, readResponseBody(buf[:n])
}

// handleWrite handles an SMB2 WRITE request ([MS-SMB2] 2.2.21).
func (c *conn) handleWrite(h header, body []byte) (uint32, []byte) {
	if len(body) < 48 {
		return statusInvalidParameter, errorResponseBody()
	}
	dataOffset := int(le.Uint16(body[2:4])) - smb2HeaderSize
	length := le.Uint32(body[4:8])
	offset := le.Uint64(body[8:16])
	of := c.getHandle(body[16:32])
	if of == nil || of.handle == nil {
		return statusInvalidParameter, errorResponseBody()
	}
	// length stays uint32 and is bounded before any int conversion so it cannot
	// wrap negative and defeat the bounds check on 32-bit builds.
	if length > maxIOSize || dataOffset < 0 || dataOffset+int(length) > len(body) {
		return statusInvalidParameter, errorResponseBody()
	}

	n, err := of.handle.WriteAt(body[dataOffset:dataOffset+int(length)], int64(offset))
	if err != nil {
		return mapVFSError(err), errorResponseBody()
	}
	return statusSuccess, writeResponseBody(uint32(n))
}

// handleFlush handles an SMB2 FLUSH request ([MS-SMB2] 2.2.17).
func (c *conn) handleFlush(h header, body []byte) (uint32, []byte) {
	if len(body) < 24 {
		return statusInvalidParameter, errorResponseBody()
	}
	if of := c.getHandle(body[8:24]); of != nil && of.handle != nil {
		if err := of.handle.Flush(); err != nil {
			return mapVFSError(err), errorResponseBody()
		}
	}
	return statusSuccess, flushResponseBody()
}

// handleClose handles an SMB2 CLOSE request ([MS-SMB2] 2.2.15). If the open was
// marked delete-on-close, the node is removed.
func (c *conn) handleClose(h header, body []byte) (uint32, []byte) {
	if len(body) < 24 {
		return statusInvalidParameter, errorResponseBody()
	}
	of := c.removeHandle(body[8:24])
	if of != nil {
		if of.handle != nil {
			// Close finalises a streaming upload (cache-mode off/writes); a failure
			// here means the write did not land, so report it instead of SUCCESS.
			if err := of.handle.Close(); err != nil {
				return mapVFSError(err), errorResponseBody()
			}
		}
		if of.deleteOnClose && of.node != nil {
			if err := of.node.Remove(); err != nil {
				return mapVFSError(err), errorResponseBody()
			}
		}
	}
	return statusSuccess, closeResponseBody()
}

// readResponseBody builds the READ response ([MS-SMB2] 2.2.20) carrying data.
func readResponseBody(data []byte) []byte {
	body := make([]byte, 16+len(data))
	le.PutUint16(body[0:2], 17) // StructureSize (fixed magic value)
	// DataOffset is measured from the start of the SMB2 header. The data sits
	// at body offset 16, i.e. wire offset 64+16 = 80. The client validates
	// DataOffset >= 80 and then subtracts 64 to locate the data in the body.
	body[2] = smb2HeaderSize + 16
	le.PutUint32(body[4:8], uint32(len(data)))
	copy(body[16:], data)
	return body
}

// writeResponseBody builds the WRITE response ([MS-SMB2] 2.2.22).
func writeResponseBody(count uint32) []byte {
	body := make([]byte, 17)
	le.PutUint16(body[0:2], 17)
	le.PutUint32(body[4:8], count)
	return body
}

// flushResponseBody builds the FLUSH response ([MS-SMB2] 2.2.18).
func flushResponseBody() []byte {
	b := make([]byte, 4)
	le.PutUint16(b[0:2], 4)
	return b
}
