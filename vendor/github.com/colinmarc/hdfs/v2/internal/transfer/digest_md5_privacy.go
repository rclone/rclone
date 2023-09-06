package transfer

import (
	"bytes"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rc4"
	"encoding/binary"
	"errors"
	"hash"
	"io"
	"net"
	"syscall"
	"time"
)

type digestMD5PrivacyConn struct {
	conn         net.Conn
	readDeadline time.Time

	sendSeqNum int
	readSeqNum int

	decodeMAC hash.Hash
	encodeMAC hash.Hash

	decryptor *rc4.Cipher
	encryptor *rc4.Cipher

	readBuf  bytes.Buffer
	writeBuf bytes.Buffer
}

// digestMD5PrivacyConn returns a net.Conn wrapper that peforms md5-digest
// encryption on data passing over it.
func newDigestMD5PrivacyConn(conn net.Conn, kic, kis, kcc, kcs []byte) digestMD5Conn {
	encryptor, _ := rc4.NewCipher(kcc)
	decryptor, _ := rc4.NewCipher(kcs)

	return &digestMD5PrivacyConn{
		conn:      conn,
		encryptor: encryptor,
		decryptor: decryptor,
		decodeMAC: hmac.New(md5.New, kis),
		encodeMAC: hmac.New(md5.New, kic),
	}
}

func (d *digestMD5PrivacyConn) Close() error {
	return d.conn.Close()
}

func (d *digestMD5PrivacyConn) LocalAddr() net.Addr {
	return d.conn.LocalAddr()
}

func (d *digestMD5PrivacyConn) RemoteAddr() net.Addr {
	return d.conn.RemoteAddr()
}

func (d *digestMD5PrivacyConn) SetDeadline(t time.Time) error {
	d.readDeadline = t
	return d.conn.SetDeadline(t)
}

func (d *digestMD5PrivacyConn) SetReadDeadline(t time.Time) error {
	d.readDeadline = t
	return d.conn.SetReadDeadline(t)
}

func (d *digestMD5PrivacyConn) SetWriteDeadline(t time.Time) error {
	return d.conn.SetWriteDeadline(t)
}

func (d *digestMD5PrivacyConn) Read(b []byte) (int, error) {
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
		return 0, err
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

func (d *digestMD5PrivacyConn) decode(input []byte) (out []byte, err error) {
	inputLen := len(input)
	if inputLen < saslIntegrityPrefixLength {
		return nil, errors.New("invalid response from datanode: bad response length")
	}

	seqNumStart := inputLen - macSeqNumLen
	msgTypeStart := seqNumStart - macMsgTypeLen

	encryptedLen := inputLen - macMsgTypeLen - macSeqNumLen
	d.decryptor.XORKeyStream(input[:encryptedLen], input[:encryptedLen])

	origHash := input[encryptedLen-macHMACLen : encryptedLen]
	encryptedLen -= macHMACLen

	seqBuf := lenEncodeBytes(d.readSeqNum)
	hmac := msgHMAC(d.decodeMAC, seqBuf, input[:encryptedLen])

	msgType := input[msgTypeStart : msgTypeStart+macMsgTypeLen]
	seqNum := input[seqNumStart : seqNumStart+macSeqNumLen]

	if !bytes.Equal(hmac, origHash) || !bytes.Equal(macMsgType[:], msgType) || !bytes.Equal(seqNum, seqBuf[:]) {
		return nil, errors.New("invalid response from datanode: HMAC check failed")
	}

	d.readSeqNum++
	return input[:encryptedLen], nil
}

func (d *digestMD5PrivacyConn) Write(b []byte) (int, error) {
	inputLen := len(b)
	seqBuf := lenEncodeBytes(d.sendSeqNum)

	encryptedLen := inputLen + macHMACLen
	outputLen := macDataLen + encryptedLen + macMsgTypeLen + macSeqNumLen
	d.writeBuf.Reset()
	d.writeBuf.Grow(outputLen)

	finalLength := encryptedLen + macMsgTypeLen + macSeqNumLen
	binary.Write(&d.writeBuf, binary.BigEndian, int32(finalLength))
	d.writeBuf.Write(b)

	hmac := msgHMAC(d.encodeMAC, seqBuf, b)
	d.writeBuf.Write(hmac)

	toEncrypt := d.writeBuf.Bytes()[macDataLen:]
	d.encryptor.XORKeyStream(toEncrypt, toEncrypt)
	d.writeBuf.Write(macMsgType[:])
	binary.Write(&d.writeBuf, binary.BigEndian, int32(d.sendSeqNum))

	d.sendSeqNum++
	n, err := d.writeBuf.WriteTo(d.conn)
	return int(n), err
}
