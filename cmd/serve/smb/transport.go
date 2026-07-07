package smb

import (
	"encoding/binary"
	"fmt"
	"io"
)

// maxMessageSize is the largest SMB2 message we will accept over the Direct
// TCP transport. SMB2 messages are negotiated to be much smaller than this,
// so it only acts as a sanity limit to protect against malicious clients.
const maxMessageSize = 16 * 1024 * 1024

// readMessage reads a single SMB2 message framed using the Direct TCP
// transport (MS-SMB2 §2.1, RFC 1002 style framing): a 4-byte header where the
// first byte is zero and the next three bytes are the big-endian length of the
// message that follows.
func readMessage(r io.Reader) ([]byte, error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return nil, err
	}
	// The high byte must be zero; the length is a 24-bit big-endian value.
	n := uint32(hdr[1])<<16 | uint32(hdr[2])<<8 | uint32(hdr[3])
	if n == 0 {
		return []byte{}, nil
	}
	if n > maxMessageSize {
		return nil, fmt.Errorf("smb: incoming message too large: %d bytes", n)
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// writeMessage writes a single SMB2 message using the Direct TCP transport
// framing described above.
func writeMessage(w io.Writer, msg []byte) error {
	n := len(msg)
	if n > maxMessageSize {
		return fmt.Errorf("smb: outgoing message too large: %d bytes", n)
	}
	var hdr [4]byte
	// hdr[0] stays zero; the length is a 24-bit big-endian value.
	hdr[1] = byte(n >> 16)
	hdr[2] = byte(n >> 8)
	hdr[3] = byte(n)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	_, err := w.Write(msg)
	return err
}

// le is a convenience alias for the little-endian byte order used throughout
// the SMB2 wire format.
var le = binary.LittleEndian
