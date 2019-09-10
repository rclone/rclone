package ftp

import "io"

type debugWrapper struct {
	conn io.ReadWriteCloser
	io.Reader
	io.Writer
}

func newDebugWrapper(conn io.ReadWriteCloser, w io.Writer) io.ReadWriteCloser {
	return &debugWrapper{
		Reader: io.TeeReader(conn, w),
		Writer: io.MultiWriter(w, conn),
		conn:   conn,
	}
}

func (w *debugWrapper) Close() error {
	return w.conn.Close()
}
