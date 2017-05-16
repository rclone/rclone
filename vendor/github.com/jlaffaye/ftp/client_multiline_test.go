package ftp

import (
	"net"
	"net/textproto"
	"reflect"
	"strings"
	"sync"
	"testing"
)

type ftpMock struct {
	listener net.Listener
	commands []string // list of received commands
	sync.WaitGroup
}

func newFtpMock(t *testing.T, addresss string) *ftpMock {
	var err error
	mock := &ftpMock{}
	mock.listener, err = net.Listen("tcp", addresss)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		// Listen for an incoming connection.
		conn, err := mock.listener.Accept()
		if err != nil {
			t.Fatal(err)
		}

		mock.Add(1)
		defer mock.Done()
		defer conn.Close()

		proto := textproto.NewConn(conn)
		proto.Writer.PrintfLine("220 FTP Server ready.")

		for {
			command, _ := proto.ReadLine()

			// Strip the arguments
			if i := strings.Index(command, " "); i > 0 {
				command = command[:i]
			}

			// Append to list of received commands
			mock.commands = append(mock.commands, command)

			// At least one command must have a multiline response
			switch command {
			case "FEAT":
				proto.Writer.PrintfLine("211-Features:\r\nFEAT\r\nPASV\r\nSIZE\r\n211 End")
			case "USER":
				proto.Writer.PrintfLine("331 Please send your password")
			case "PASS":
				proto.Writer.PrintfLine("230-Hey,\r\nWelcome to my FTP\r\n230 Access granted")
			case "TYPE":
				proto.Writer.PrintfLine("200 Type set ok")
			case "QUIT":
				proto.Writer.PrintfLine("221 Goodbye.")
				return
			default:
				t.Fatal("unknown command:", command)
			}
		}
	}()

	return mock
}

// Closes the listening socket
func (mock *ftpMock) Close() {
	mock.listener.Close()
}

// ftp.mozilla.org uses multiline 220 response
func TestMultiline(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping test in short mode.")
	}

	address := "localhost:2121"
	mock := newFtpMock(t, address)
	defer mock.Close()

	c, err := Dial(address)
	if err != nil {
		t.Fatal(err)
	}

	err = c.Login("anonymous", "anonymous")
	if err != nil {
		t.Fatal(err)
	}

	c.Quit()

	// Wait for the connection to close
	mock.Wait()

	expected := []string{"FEAT", "USER", "PASS", "TYPE", "QUIT"}
	if !reflect.DeepEqual(mock.commands, expected) {
		t.Fatal("unexpected sequence of commands:", mock.commands, "expected:", expected)
	}
}
