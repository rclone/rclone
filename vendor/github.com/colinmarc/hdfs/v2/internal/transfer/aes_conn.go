package transfer

import (
	"crypto/aes"
	"crypto/cipher"
	"net"
	"time"
)

type aesConn struct {
	conn net.Conn

	encStream cipher.StreamWriter
	decStream cipher.StreamReader
}

func newAesConn(conn net.Conn, inKey, outKey, inIv, outIv []byte) (net.Conn, error) {
	c := &aesConn{conn: conn}

	encBlock, err := aes.NewCipher(inKey)
	if err != nil {
		return nil, err
	}

	decBlock, err := aes.NewCipher(outKey)
	if err != nil {
		return nil, err
	}

	c.encStream = cipher.StreamWriter{S: cipher.NewCTR(encBlock, inIv), W: conn}
	c.decStream = cipher.StreamReader{S: cipher.NewCTR(decBlock, outIv), R: conn}
	return c, nil
}

func (d *aesConn) Close() error {
	return d.conn.Close()
}

func (d *aesConn) LocalAddr() net.Addr {
	return d.conn.LocalAddr()
}

func (d *aesConn) RemoteAddr() net.Addr {
	return d.conn.RemoteAddr()
}

func (d *aesConn) SetDeadline(t time.Time) error {
	return d.conn.SetDeadline(t)
}

func (d *aesConn) SetReadDeadline(t time.Time) error {
	return d.conn.SetReadDeadline(t)
}

func (d *aesConn) SetWriteDeadline(t time.Time) error {
	return d.conn.SetWriteDeadline(t)
}

func (d *aesConn) Write(b []byte) (n int, err error) {
	return d.encStream.Write(b)
}

func (d *aesConn) Read(b []byte) (n int, err error) {
	return d.decStream.Read(b)
}
