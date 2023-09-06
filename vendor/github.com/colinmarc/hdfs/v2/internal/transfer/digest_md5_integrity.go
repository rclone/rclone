package transfer

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"encoding/binary"
	"errors"
	"fmt"
	"hash"
	"io"
	"net"
	"syscall"
	"time"
)

// digestMD5IntegrityConn returns a net.Conn wrapper that peforms md5-digest
// integrity checks on data passing over it.
type digestMD5IntegrityConn struct {
	conn         net.Conn
	readDeadline time.Time

	readBuf  bytes.Buffer
	writeBuf bytes.Buffer

	sendSeqNum int
	readSeqNum int

	encodeMAC hash.Hash
	decodeMAC hash.Hash
}

func newDigestMD5IntegrityConn(conn net.Conn, kic, kis []byte) digestMD5Conn {
	return &digestMD5IntegrityConn{
		conn:      conn,
		encodeMAC: hmac.New(md5.New, kic),
		decodeMAC: hmac.New(md5.New, kis),
	}
}

func (d *digestMD5IntegrityConn) Close() error {
	return d.conn.Close()
}

func (d *digestMD5IntegrityConn) LocalAddr() net.Addr {
	return d.conn.LocalAddr()
}

func (d *digestMD5IntegrityConn) RemoteAddr() net.Addr {
	return d.conn.RemoteAddr()
}

func (d *digestMD5IntegrityConn) SetDeadline(t time.Time) error {
	d.readDeadline = t
	return d.conn.SetDeadline(t)
}

func (d *digestMD5IntegrityConn) SetReadDeadline(t time.Time) error {
	d.readDeadline = t
	return d.conn.SetReadDeadline(t)
}

func (d *digestMD5IntegrityConn) SetWriteDeadline(t time.Time) error {
	return d.conn.SetWriteDeadline(t)
}

func (d *digestMD5IntegrityConn) Write(b []byte) (n int, err error) {
	inputLen := len(b)
	seqBuf := lenEncodeBytes(d.sendSeqNum)
	outputLen := macDataLen + inputLen + macHMACLen + macMsgTypeLen + macSeqNumLen

	d.writeBuf.Reset()
	d.writeBuf.Grow(outputLen)

	binary.Write(&d.writeBuf, binary.BigEndian, int32(outputLen-macDataLen))
	d.writeBuf.Write(b)

	hmac := msgHMAC(d.encodeMAC, seqBuf, b)
	d.writeBuf.Write(hmac)
	d.writeBuf.Write(macMsgType[:])
	binary.Write(&d.writeBuf, binary.BigEndian, int32(d.sendSeqNum))

	d.sendSeqNum++
	wr, err := d.writeBuf.WriteTo(d.conn)
	return int(wr), err
}

// Read will decode the underlying bytes and then copy them from our
// buffer into the provided byte slice
func (d *digestMD5IntegrityConn) Read(b []byte) (int, error) {
	if !d.readDeadline.IsZero() && d.readDeadline.Before(time.Now()) {
		return 0, syscall.ETIMEDOUT
	}

	n, err := d.readBuf.Read(b)
	if len(b) == n || (err != nil && err != io.EOF) {
		return n, err
	}

	var sz int32
	err = binary.Read(d.conn, binary.BigEndian, &sz)
	if err != nil {
		return n, err
	}

	d.readBuf.Reset()
	d.readBuf.Grow(int(sz))
	_, err = io.CopyN(&d.readBuf, d.conn, int64(sz))
	if err != nil {
		return n, err
	}

	decoded, err := d.decode(d.readBuf.Bytes())
	if err != nil {
		return n, err
	}

	d.readBuf.Truncate(len(decoded))
	return d.readBuf.Read(b[n:])
}

// decode will decode a message from the server and perform the integrity
// protection check, removing the verification and mac data in what is returned
// the slice returned is an alias to the buffer and must be either used or
// copied to a new slice before calling decode again
func (d *digestMD5IntegrityConn) decode(input []byte) ([]byte, error) {
	inputLen := len(input)
	if inputLen < saslIntegrityPrefixLength {
		return nil, fmt.Errorf("Input length smaller than the integrity prefix")
	}

	seqBuf := lenEncodeBytes(d.readSeqNum)

	dataLen := inputLen - macHMACLen - macMsgTypeLen - macSeqNumLen
	hmac := msgHMAC(d.decodeMAC, seqBuf, input[:dataLen])

	seqNumStart := inputLen - macSeqNumLen
	msgTypeStart := seqNumStart - macMsgTypeLen
	origHashStart := msgTypeStart - macHMACLen

	if !bytes.Equal(hmac, input[origHashStart:origHashStart+macHMACLen]) ||
		!bytes.Equal(macMsgType[:], input[msgTypeStart:msgTypeStart+macMsgTypeLen]) ||
		!bytes.Equal(seqBuf[:], input[seqNumStart:seqNumStart+macSeqNumLen]) {
		return nil, errors.New("HMAC Integrity Check failed")
	}

	d.readSeqNum++
	return input[:dataLen], nil
}

// msgHMAC implements the HMAC wrapper per the RFC:
//
//     HMAC(ki, {seqnum, msg})[0..9].
func msgHMAC(mac hash.Hash, seq [4]byte, msg []byte) []byte {
	mac.Reset()
	mac.Write(seq[:])
	mac.Write(msg)

	return mac.Sum(nil)[:10]
}
