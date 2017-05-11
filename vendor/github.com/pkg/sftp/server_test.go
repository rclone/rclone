package sftp

import (
	"io"
	"sync"
	"testing"
)

func clientServerPair(t *testing.T) (*Client, *Server) {
	cr, sw := io.Pipe()
	sr, cw := io.Pipe()
	server, err := NewServer(struct {
		io.Reader
		io.WriteCloser
	}{sr, sw})
	if err != nil {
		t.Fatal(err)
	}
	go server.Serve()
	client, err := NewClientPipe(cr, cw)
	if err != nil {
		t.Fatalf("%+v\n", err)
	}
	return client, server
}

type sshFxpTestBadExtendedPacket struct {
	ID        uint32
	Extension string
	Data      string
}

func (p sshFxpTestBadExtendedPacket) id() uint32 { return p.ID }

func (p sshFxpTestBadExtendedPacket) MarshalBinary() ([]byte, error) {
	l := 1 + 4 + 4 + // type(byte) + uint32 + uint32
		len(p.Extension) +
		len(p.Data)

	b := make([]byte, 0, l)
	b = append(b, ssh_FXP_EXTENDED)
	b = marshalUint32(b, p.ID)
	b = marshalString(b, p.Extension)
	b = marshalString(b, p.Data)
	return b, nil
}

// test that errors are sent back when we request an invalid extended packet operation
func TestInvalidExtendedPacket(t *testing.T) {
	client, server := clientServerPair(t)
	defer client.Close()
	defer server.Close()

	badPacket := sshFxpTestBadExtendedPacket{client.nextID(), "thisDoesn'tExist", "foobar"}
	_, _, err := client.clientConn.sendPacket(badPacket)
	if err == nil {
		t.Fatal("expected error from bad packet")
	}

	// try to stat a file; the client should have shut down.
	filePath := "/etc/passwd"
	_, err = client.Stat(filePath)
	if err == nil {
		t.Fatal("expected error from closed connection")
	}

}

// test that server handles concurrent requests correctly
func TestConcurrentRequests(t *testing.T) {
	client, server := clientServerPair(t)
	defer client.Close()
	defer server.Close()

	concurrency := 2
	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()

			for j := 0; j < 1024; j++ {
				f, err := client.Open("/etc/passwd")
				if err != nil {
					t.Errorf("failed to open file: %v", err)
				}
				if err := f.Close(); err != nil {
					t.Errorf("failed t close file: %v", err)
				}
			}
		}()
	}
	wg.Wait()
}
