package sftp

import (
	"io"
	"os"
	"regexp"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
)

const (
	typeDirectory = "d"
	typeFile      = "[^d]"
)

func TestRunLsWithExamplesDirectory(t *testing.T) {
	path := "examples"
	item, _ := os.Stat(path)
	result := runLs(path, item)
	runLsTestHelper(t, result, typeDirectory, path)
}

func TestRunLsWithLicensesFile(t *testing.T) {
	path := "LICENSE"
	item, _ := os.Stat(path)
	result := runLs(path, item)
	runLsTestHelper(t, result, typeFile, path)
}

/*
   The format of the `longname' field is unspecified by this protocol.
   It MUST be suitable for use in the output of a directory listing
   command (in fact, the recommended operation for a directory listing
   command is to simply display this data).  However, clients SHOULD NOT
   attempt to parse the longname field for file attributes; they SHOULD
   use the attrs field instead.

    The recommended format for the longname field is as follows:

        -rwxr-xr-x   1 mjos     staff      348911 Mar 25 14:29 t-filexfer
        1234567890 123 12345678 12345678 12345678 123456789012

   Here, the first line is sample output, and the second field indicates
   widths of the various fields.  Fields are separated by spaces.  The
   first field lists file permissions for user, group, and others; the
   second field is link count; the third field is the name of the user
   who owns the file; the fourth field is the name of the group that
   owns the file; the fifth field is the size of the file in bytes; the
   sixth field (which actually may contain spaces, but is fixed to 12
   characters) is the file modification time, and the seventh field is
   the file name.  Each field is specified to be a minimum of certain
   number of character positions (indicated by the second line above),
   but may also be longer if the data does not fit in the specified
   length.

    The SSH_FXP_ATTRS response has the following format:

        uint32     id
        ATTRS      attrs

   where `id' is the request identifier, and `attrs' is the returned
   file attributes as described in Section ``File Attributes''.
*/
func runLsTestHelper(t *testing.T, result, expectedType, path string) {
	// using regular expressions to make tests work on all systems
	// a virtual file system (like afero) would be needed to mock valid filesystem checks
	// expected layout is:
	// drwxr-xr-x   8 501      20            272 Aug  9 19:46 examples

	// permissions (len 10, "drwxr-xr-x")
	got := result[0:10]
	if ok, err := regexp.MatchString("^"+expectedType+"[rwx-]{9}$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): permission field mismatch, expected dir, got: %#v, err: %#v", path, got, err)
	}

	// space
	got = result[10:11]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 1 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// link count (len 3, number)
	got = result[12:15]
	if ok, err := regexp.MatchString("^\\s*[0-9]+$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): link count field mismatch, got: %#v, err: %#v", path, got, err)
	}

	// spacer
	got = result[15:16]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 2 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// username / uid (len 8, number or string)
	got = result[16:24]
	if ok, err := regexp.MatchString("^[^\\s]{1,8}\\s*$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): username / uid mismatch, expected user, got: %#v, err: %#v", path, got, err)
	}

	// spacer
	got = result[24:25]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 3 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// groupname / gid (len 8, number or string)
	got = result[25:33]
	if ok, err := regexp.MatchString("^[^\\s]{1,8}\\s*$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): groupname / gid mismatch, expected group, got: %#v, err: %#v", path, got, err)
	}

	// spacer
	got = result[33:34]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 4 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// filesize (len 8)
	got = result[34:42]
	if ok, err := regexp.MatchString("^\\s*[0-9]+$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): filesize field mismatch, expected size in bytes, got: %#v, err: %#v", path, got, err)
	}

	// spacer
	got = result[42:43]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 5 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// mod time (len 12, e.g. Aug  9 19:46)
	got = result[43:55]
	layout := "Jan  2 15:04"
	_, err := time.Parse(layout, got)

	if err != nil {
		layout = "Jan  2 2006"
		_, err = time.Parse(layout, got)
	}
	if err != nil {
		t.Errorf("runLs(%#v, *FileInfo): mod time field mismatch, expected date layout %s, got: %#v, err: %#v", path, layout, got, err)
	}

	// spacer
	got = result[55:56]
	if ok, err := regexp.MatchString("^\\s$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): spacer 6 mismatch, expected whitespace, got: %#v, err: %#v", path, got, err)
	}

	// filename
	got = result[56:]
	if ok, err := regexp.MatchString("^"+path+"$", got); !ok {
		t.Errorf("runLs(%#v, *FileInfo): name field mismatch, expected examples, got: %#v, err: %#v", path, got, err)
	}
}

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
// this validates the following rfc draft is followed https://tools.ietf.org/html/draft-ietf-secsh-filexfer-extensions-00
func TestInvalidExtendedPacket(t *testing.T) {
	client, server := clientServerPair(t)
	defer client.Close()
	defer server.Close()

	badPacket := sshFxpTestBadExtendedPacket{client.nextID(), "thisDoesn'tExist", "foobar"}
	typ, data, err := client.clientConn.sendPacket(badPacket)
	if err != nil {
		t.Fatalf("unexpected error from sendPacket: %s", err)
	}
	if typ != ssh_FXP_STATUS {
		t.Fatalf("received non-FPX_STATUS packet: %v", typ)
	}

	err = unmarshalStatus(badPacket.id(), data)
	statusErr, ok := err.(*StatusError)
	if !ok {
		t.Fatal("failed to convert error from unmarshalStatus to *StatusError")
	}
	if statusErr.Code != ssh_FX_OP_UNSUPPORTED {
		t.Errorf("statusErr.Code => %d, wanted %d", statusErr.Code, ssh_FX_OP_UNSUPPORTED)
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

// Test error conversion
func TestStatusFromError(t *testing.T) {
	type test struct {
		err error
		pkt sshFxpStatusPacket
	}
	tpkt := func(id, code uint32) sshFxpStatusPacket {
		return sshFxpStatusPacket{
			ID:          id,
			StatusError: StatusError{Code: code},
		}
	}
	test_cases := []test{
		test{syscall.ENOENT, tpkt(1, ssh_FX_NO_SUCH_FILE)},
		test{&os.PathError{Err: syscall.ENOENT},
			tpkt(2, ssh_FX_NO_SUCH_FILE)},
		test{&os.PathError{Err: errors.New("foo")}, tpkt(3, ssh_FX_FAILURE)},
		test{ErrSshFxEof, tpkt(4, ssh_FX_EOF)},
		test{ErrSshFxOpUnsupported, tpkt(5, ssh_FX_OP_UNSUPPORTED)},
		test{io.EOF, tpkt(6, ssh_FX_EOF)},
		test{os.ErrNotExist, tpkt(7, ssh_FX_NO_SUCH_FILE)},
	}
	for _, tc := range test_cases {
		tc.pkt.StatusError.msg = tc.err.Error()
		assert.Equal(t, tc.pkt, statusFromError(tc.pkt, tc.err))
	}
}
