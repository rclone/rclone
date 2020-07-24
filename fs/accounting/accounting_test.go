package accounting

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/asyncreader"
	"github.com/rclone/rclone/fs/fserrors"
	"github.com/rclone/rclone/lib/readers"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Check it satisfies the interfaces
var (
	_ io.ReadCloser = &Account{}
	_ io.WriterTo   = &Account{}
	_ io.Reader     = &accountStream{}
	_ Accounter     = &Account{}
	_ Accounter     = &accountStream{}
)

func TestNewAccountSizeName(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 1, "test")
	assert.Equal(t, in, acc.in)
	assert.Equal(t, acc, stats.inProgress.get("test"))
	err := acc.Close()
	assert.NoError(t, err)
	assert.Equal(t, acc, stats.inProgress.get("test"))
	acc.Done()
	assert.Nil(t, stats.inProgress.get("test"))
	assert.False(t, acc.HasBuffer())
}

func TestAccountWithBuffer(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))

	stats := NewStats()
	acc := newAccountSizeName(stats, in, -1, "test")
	assert.False(t, acc.HasBuffer())
	acc.WithBuffer()
	assert.True(t, acc.HasBuffer())
	// should have a buffer for an unknown size
	_, ok := acc.in.(*asyncreader.AsyncReader)
	require.True(t, ok)
	assert.NoError(t, acc.Close())

	acc = newAccountSizeName(stats, in, 1, "test")
	acc.WithBuffer()
	// should not have a buffer for a small size
	_, ok = acc.in.(*asyncreader.AsyncReader)
	require.False(t, ok)
	assert.NoError(t, acc.Close())
}

func TestAccountGetUpdateReader(t *testing.T) {
	test := func(doClose bool) func(t *testing.T) {
		return func(t *testing.T) {
			in := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
			stats := NewStats()
			acc := newAccountSizeName(stats, in, 1, "test")

			assert.Equal(t, in, acc.GetReader())
			assert.Equal(t, acc, stats.inProgress.get("test"))

			if doClose {
				// close the account before swapping it out
				require.NoError(t, acc.Close())
			}

			in2 := ioutil.NopCloser(bytes.NewBuffer([]byte{1}))
			acc.UpdateReader(in2)

			assert.Equal(t, in2, acc.GetReader())
			assert.Equal(t, acc, stats.inProgress.get("test"))

			assert.NoError(t, acc.Close())
		}
	}
	t.Run("NoClose", test(false))
	t.Run("Close", test(true))
}

func TestAccountRead(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 1, "test")

	assert.True(t, acc.values.start.IsZero())
	acc.values.mu.Lock()
	assert.Equal(t, 0, acc.values.lpBytes)
	assert.Equal(t, int64(0), acc.values.bytes)
	acc.values.mu.Unlock()
	assert.Equal(t, int64(0), stats.bytes)

	var buf = make([]byte, 2)
	n, err := acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte{1, 2}, buf[:n])

	assert.False(t, acc.values.start.IsZero())
	acc.values.mu.Lock()
	assert.Equal(t, 2, acc.values.lpBytes)
	assert.Equal(t, int64(2), acc.values.bytes)
	acc.values.mu.Unlock()
	assert.Equal(t, int64(2), stats.bytes)

	n, err = acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, []byte{3}, buf[:n])

	n, err = acc.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	assert.NoError(t, acc.Close())
}

func testAccountWriteTo(t *testing.T, withBuffer bool) {
	buf := make([]byte, 2*asyncreader.BufferSize+1)
	for i := range buf {
		buf[i] = byte(i % 251)
	}
	in := ioutil.NopCloser(bytes.NewBuffer(buf))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, int64(len(buf)), "test")
	if withBuffer {
		acc = acc.WithBuffer()
	}

	assert.True(t, acc.values.start.IsZero())
	acc.values.mu.Lock()
	assert.Equal(t, 0, acc.values.lpBytes)
	assert.Equal(t, int64(0), acc.values.bytes)
	acc.values.mu.Unlock()
	assert.Equal(t, int64(0), stats.bytes)

	var out bytes.Buffer

	n, err := acc.WriteTo(&out)
	assert.NoError(t, err)
	assert.Equal(t, int64(len(buf)), n)
	assert.Equal(t, buf, out.Bytes())

	assert.False(t, acc.values.start.IsZero())
	acc.values.mu.Lock()
	assert.Equal(t, len(buf), acc.values.lpBytes)
	assert.Equal(t, int64(len(buf)), acc.values.bytes)
	acc.values.mu.Unlock()
	assert.Equal(t, int64(len(buf)), stats.bytes)

	assert.NoError(t, acc.Close())
}

func TestAccountWriteTo(t *testing.T) {
	testAccountWriteTo(t, false)
}

func TestAccountWriteToWithBuffer(t *testing.T) {
	testAccountWriteTo(t, true)
}

func TestAccountString(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 3, "test")

	// FIXME not an exhaustive test!

	assert.Equal(t, "test:  0% /3, 0/s, -", strings.TrimSpace(acc.String()))

	var buf = make([]byte, 2)
	n, err := acc.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)

	assert.Equal(t, "test: 66% /3, 0/s, -", strings.TrimSpace(acc.String()))

	assert.NoError(t, acc.Close())
}

// Test the Accounter interface methods on Account and accountStream
func TestAccountAccounter(t *testing.T) {
	in := ioutil.NopCloser(bytes.NewBuffer([]byte{1, 2, 3}))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 3, "test")

	assert.True(t, in == acc.OldStream())

	in2 := ioutil.NopCloser(bytes.NewBuffer([]byte{2, 3, 4}))

	acc.SetStream(in2)
	assert.True(t, in2 == acc.OldStream())

	r := acc.WrapStream(in)
	as, ok := r.(Accounter)
	require.True(t, ok)
	assert.True(t, in == as.OldStream())
	assert.True(t, in2 == acc.OldStream())
	accs, ok := r.(*accountStream)
	require.True(t, ok)
	assert.Equal(t, acc, accs.acc)
	assert.True(t, in == accs.in)

	// Check Read on the accountStream
	var buf = make([]byte, 2)
	n, err := r.Read(buf)
	assert.NoError(t, err)
	assert.Equal(t, 2, n)
	assert.Equal(t, []byte{1, 2}, buf[:n])

	// Test that we can get another accountstream out
	in3 := ioutil.NopCloser(bytes.NewBuffer([]byte{3, 1, 2}))
	r2 := as.WrapStream(in3)
	as2, ok := r2.(Accounter)
	require.True(t, ok)
	assert.True(t, in3 == as2.OldStream())
	assert.True(t, in2 == acc.OldStream())
	accs2, ok := r2.(*accountStream)
	require.True(t, ok)
	assert.Equal(t, acc, accs2.acc)
	assert.True(t, in3 == accs2.in)

	// Test we can set this new accountStream
	as2.SetStream(in)
	assert.True(t, in == as2.OldStream())

	// Test UnWrap on accountStream
	unwrapped, wrap := UnWrap(r2)
	assert.True(t, unwrapped == in)
	r3 := wrap(in2)
	assert.True(t, in2 == r3.(Accounter).OldStream())

	// TestUnWrap on a normal io.Reader
	unwrapped, wrap = UnWrap(in2)
	assert.True(t, unwrapped == in2)
	assert.True(t, wrap(in3) == in3)

}

func TestAccountMaxTransfer(t *testing.T) {
	old := fs.Config.MaxTransfer
	oldMode := fs.Config.CutoffMode

	fs.Config.MaxTransfer = 15
	defer func() {
		fs.Config.MaxTransfer = old
		fs.Config.CutoffMode = oldMode
	}()

	in := ioutil.NopCloser(bytes.NewBuffer(make([]byte, 100)))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 1, "test")

	var b = make([]byte, 10)

	n, err := acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
	n, err = acc.Read(b)
	assert.Equal(t, 5, n)
	assert.Equal(t, ErrorMaxTransferLimitReachedFatal, err)
	n, err = acc.Read(b)
	assert.Equal(t, 0, n)
	assert.Equal(t, ErrorMaxTransferLimitReachedFatal, err)
	assert.True(t, fserrors.IsFatalError(err))

	fs.Config.CutoffMode = fs.CutoffModeSoft
	stats = NewStats()
	acc = newAccountSizeName(stats, in, 1, "test")

	n, err = acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
	n, err = acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
	n, err = acc.Read(b)
	assert.Equal(t, 10, n)
	assert.NoError(t, err)
}

func TestAccountMaxTransferWriteTo(t *testing.T) {
	old := fs.Config.MaxTransfer
	oldMode := fs.Config.CutoffMode

	fs.Config.MaxTransfer = 15
	defer func() {
		fs.Config.MaxTransfer = old
		fs.Config.CutoffMode = oldMode
	}()

	in := ioutil.NopCloser(readers.NewPatternReader(1024))
	stats := NewStats()
	acc := newAccountSizeName(stats, in, 1, "test")

	var b bytes.Buffer

	n, err := acc.WriteTo(&b)
	assert.Equal(t, int64(15), n)
	assert.Equal(t, ErrorMaxTransferLimitReachedFatal, err)
}

func TestShortenName(t *testing.T) {
	for _, test := range []struct {
		in   string
		size int
		want string
	}{
		{"", 0, ""},
		{"abcde", 10, "abcde"},
		{"abcde", 0, "abcde"},
		{"abcde", -1, "abcde"},
		{"abcde", 5, "abcde"},
		{"abcde", 4, "ab…e"},
		{"abcde", 3, "a…e"},
		{"abcde", 2, "a…"},
		{"abcde", 1, "…"},
		{"abcdef", 6, "abcdef"},
		{"abcdef", 5, "ab…ef"},
		{"abcdef", 4, "ab…f"},
		{"abcdef", 3, "a…f"},
		{"abcdef", 2, "a…"},
		{"áßcdèf", 1, "…"},
		{"áßcdè", 5, "áßcdè"},
		{"áßcdè", 4, "áß…è"},
		{"áßcdè", 3, "á…è"},
		{"áßcdè", 2, "á…"},
		{"áßcdè", 1, "…"},
		{"áßcdèł", 6, "áßcdèł"},
		{"áßcdèł", 5, "áß…èł"},
		{"áßcdèł", 4, "áß…ł"},
		{"áßcdèł", 3, "á…ł"},
		{"áßcdèł", 2, "á…"},
		{"áßcdèł", 1, "…"},
	} {
		t.Run(fmt.Sprintf("in=%q, size=%d", test.in, test.size), func(t *testing.T) {
			got := shortenName(test.in, test.size)
			assert.Equal(t, test.want, got)
			if test.size > 0 {
				assert.True(t, utf8.RuneCountInString(got) <= test.size, "too big")
			}
		})
	}
}
