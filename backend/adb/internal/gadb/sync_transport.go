package gadb

// sync_transport.go is vendored from github.com/electricbubble/gadb at
// upstream commit 2e108649b dated 2025-03-14, MIT licensed. The
// ReadDirectoryEntry function is modified to call fixupMode (see mode.go)
// for correct os.FileMode bit translation; the modification site carries
// its own comment.

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

type syncTransport struct {
	sock        net.Conn
	readTimeout time.Duration
}

func newSyncTransport(sock net.Conn, readTimeout time.Duration) syncTransport {
	return syncTransport{sock: sock, readTimeout: readTimeout}
}

func (sync syncTransport) Send(command, data string) (err error) {
	if len(command) != 4 {
		return errors.New("sync commands must have length 4")
	}
	msg := bytes.NewBufferString(command)
	if err = binary.Write(msg, binary.LittleEndian, int32(len(data))); err != nil {
		return fmt.Errorf("sync transport write: %w", err)
	}
	msg.WriteString(data)

	debugLog(fmt.Sprintf("--> %s", msg.String()))
	return _send(sync.sock, msg.Bytes())
}

// syncMaxChunkSize is the maximum byte count per ADB SYNC DATA frame, set
// by the SYNC protocol itself. Larger writes must be split.
const syncMaxChunkSize = 64 * 1024

func (sync syncTransport) SendStream(reader io.Reader) (err error) {
	// Allocate the chunk buffer once per stream; reuse across iterations.
	// The previous implementation allocated 64 KiB per iteration, which on
	// a 1 GiB Push generates ~16k garbage allocations and corresponding
	// GC pressure.
	//
	// io.Reader may return n>0 with io.EOF in the same call (Go io.Reader
	// contract). The previous control flow dropped that final chunk by
	// breaking on EOF before calling sendChunk. Always send buf[:n] when
	// n>0, then check the error. Found via end-to-end device push 2026-05-08;
	// also present in upstream electricbubble/gadb at commit 2e108649b.
	buf := make([]byte, syncMaxChunkSize)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if sendErr := sync.sendChunk(buf[:n]); sendErr != nil {
				return sendErr
			}
		}
		if readErr == io.EOF {
			return nil
		}
		if readErr != nil {
			return readErr
		}
	}
}

func (sync syncTransport) SendStatus(statusCode string, n uint32) (err error) {
	msg := bytes.NewBufferString(statusCode)
	if err = binary.Write(msg, binary.LittleEndian, n); err != nil {
		return fmt.Errorf("sync transport write: %w", err)
	}
	debugLog(fmt.Sprintf("--> %s", msg.String()))
	return _send(sync.sock, msg.Bytes())
}

func (sync syncTransport) sendChunk(buffer []byte) (err error) {
	msg := bytes.NewBufferString("DATA")
	if err = binary.Write(msg, binary.LittleEndian, int32(len(buffer))); err != nil {
		return fmt.Errorf("sync transport write: %w", err)
	}
	debugLog(fmt.Sprintf("--> %s ......", msg.String()))
	msg.Write(buffer)
	return _send(sync.sock, msg.Bytes())
}

func (sync syncTransport) VerifyStatus() (err error) {
	var status string
	if status, err = sync.ReadStringN(4); err != nil {
		return err
	}

	log := bytes.NewBufferString(fmt.Sprintf("<-- %s", status))
	defer func() {
		debugLog(log.String())
	}()

	var tmpUint32 uint32
	if tmpUint32, err = sync.ReadUint32(); err != nil {
		return fmt.Errorf("sync transport read (status): %w", err)
	}
	log.WriteString(fmt.Sprintf(" %d\t", tmpUint32))

	var msg string
	if msg, err = sync.ReadStringN(int(tmpUint32)); err != nil {
		return err
	}
	log.WriteString(msg)

	if status == "FAIL" {
		err = fmt.Errorf("sync verify status (fail): %s", msg)
		return
	}

	if status != "OKAY" {
		err = fmt.Errorf("sync verify status: Unknown error: %s", msg)
		return
	}

	return
}

var errSyncReadChunkDone = errors.New("sync read chunk done")

func (sync syncTransport) WriteStream(dest io.Writer) (err error) {
	var chunk []byte
	save := func() error {
		if chunk, err = sync.readChunk(); err != nil && err != errSyncReadChunkDone {
			return fmt.Errorf("sync read chunk: %w", err)
		}
		if err == errSyncReadChunkDone {
			return err
		}
		if err = _send(dest, chunk); err != nil {
			return fmt.Errorf("sync write stream: %w", err)
		}
		return nil
	}

	for err == nil {
		err = save()
	}

	if err == errSyncReadChunkDone {
		err = nil
	}
	return
}

func (sync syncTransport) readChunk() (chunk []byte, err error) {
	var status string
	if status, err = sync.ReadStringN(4); err != nil {
		return nil, err
	}

	log := bytes.NewBufferString("")
	defer func() { debugLog(log.String()) }()

	var tmpUint32 uint32
	if tmpUint32, err = sync.ReadUint32(); err != nil {
		return nil, fmt.Errorf("read chunk (length): %w", err)
	}

	if status == "FAIL" {
		log.WriteString(fmt.Sprintf("<-- %s\t%d\t", status, tmpUint32))
		var sError string
		if sError, err = sync.ReadStringN(int(tmpUint32)); err != nil {
			return nil, fmt.Errorf("read chunk (error message): %w", err)
		}
		err = fmt.Errorf("status (fail): %s", sError)
		log.WriteString(sError)
		return
	}

	switch status {
	case "DONE":
		log.WriteString(fmt.Sprintf("<-- %s", status))
		err = errSyncReadChunkDone
		return
	case "DATA":
		log.WriteString(fmt.Sprintf("<-- %s\t%d\t", status, tmpUint32))
		if chunk, err = sync.ReadBytesN(int(tmpUint32)); err != nil {
			return nil, err
		}
	default:
		log.WriteString(fmt.Sprintf("<-- %s\t%d\t", status, tmpUint32))
		err = errors.New("unknown error")
	}

	log.WriteString("......")

	return

}

// ReadDirectoryEntry reads one entry from a SYNC LIST response stream.
func (sync syncTransport) ReadDirectoryEntry() (entry DeviceFileInfo, err error) {
	var status string
	if status, err = sync.ReadStringN(4); err != nil {
		return DeviceFileInfo{}, err
	}

	log := bytes.NewBufferString(fmt.Sprintf("<-- %s", status))
	defer func() {
		debugLog(log.String())
	}()

	if status == "DONE" {
		return
	}

	log = bytes.NewBufferString(fmt.Sprintf("<-- %s\t", status))

	// Modification from upstream: read the wire mode bits as raw uint32 and
	// translate to os.FileMode via fixupMode (see mode.go). Upstream gadb at
	// commit 2e108649b reads binary.Read(sync.sock, binary.LittleEndian,
	// &entry.Mode) directly into the os.FileMode field at sync_transport.go
	// line 199. That writes the Unix mode_t type bits (S_IFDIR=0x4000,
	// S_IFLNK=0xa000, S_IFREG=0x8000) into byte positions that do not align
	// with Go's os.FileMode type bits (ModeDir=1<<31, ModeSymlink=1<<27,
	// etc.). Downstream callers then see every directory classified as a
	// regular file: rclone lsd returns empty, recursive copy is broken, and
	// rclone sync misroutes everything. The fixupMode helper performs the
	// bit-position translation.
	var rawMode uint32
	if err = binary.Read(sync.sock, binary.LittleEndian, &rawMode); err != nil {
		return DeviceFileInfo{}, fmt.Errorf("sync transport read (mode): %w", err)
	}
	entry.Mode = fixupMode(rawMode)
	log.WriteString(entry.Mode.String() + "\t")

	var wireSize uint32
	if wireSize, err = sync.ReadUint32(); err != nil {
		return DeviceFileInfo{}, fmt.Errorf("sync transport read (size): %w", err)
	}
	entry.Size = int64(wireSize)
	log.WriteString(fmt.Sprintf("%10d", entry.Size) + "\t")

	var tmpUint32 uint32
	if tmpUint32, err = sync.ReadUint32(); err != nil {
		return DeviceFileInfo{}, fmt.Errorf("sync transport read (time): %w", err)
	}
	entry.LastModified = time.Unix(int64(tmpUint32), 0)
	log.WriteString(entry.LastModified.String() + "\t")

	if tmpUint32, err = sync.ReadUint32(); err != nil {
		return DeviceFileInfo{}, fmt.Errorf("sync transport read (file name length): %w", err)
	}
	log.WriteString(fmt.Sprintf("%d\t", tmpUint32))

	if entry.Name, err = sync.ReadStringN(int(tmpUint32)); err != nil {
		return DeviceFileInfo{}, fmt.Errorf("sync transport read (file name): %w", err)
	}
	log.WriteString(entry.Name + "\t")

	return
}

func (sync syncTransport) ReadUint32() (n uint32, err error) {
	err = binary.Read(sync.sock, binary.LittleEndian, &n)
	return
}

func (sync syncTransport) ReadStringN(size int) (s string, err error) {
	var raw []byte
	if raw, err = sync.ReadBytesN(size); err != nil {
		return "", err
	}
	return string(raw), nil
}

func (sync syncTransport) ReadBytesN(size int) (raw []byte, err error) {
	_ = sync.sock.SetReadDeadline(time.Now().Add(sync.readTimeout))
	return _readN(sync.sock, size)
}

func (sync syncTransport) Close() (err error) {
	if sync.sock == nil {
		return nil
	}
	return sync.sock.Close()
}
