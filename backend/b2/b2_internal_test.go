package b2

import (
	"bytes"
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test b2 string encoding
// https://www.backblaze.com/b2/docs/string_encoding.html

var encodeTest = []struct {
	fullyEncoded     string
	minimallyEncoded string
	plainText        string
}{
	{fullyEncoded: "%20", minimallyEncoded: "+", plainText: " "},
	{fullyEncoded: "%21", minimallyEncoded: "!", plainText: "!"},
	{fullyEncoded: "%22", minimallyEncoded: "%22", plainText: "\""},
	{fullyEncoded: "%23", minimallyEncoded: "%23", plainText: "#"},
	{fullyEncoded: "%24", minimallyEncoded: "$", plainText: "$"},
	{fullyEncoded: "%25", minimallyEncoded: "%25", plainText: "%"},
	{fullyEncoded: "%26", minimallyEncoded: "%26", plainText: "&"},
	{fullyEncoded: "%27", minimallyEncoded: "'", plainText: "'"},
	{fullyEncoded: "%28", minimallyEncoded: "(", plainText: "("},
	{fullyEncoded: "%29", minimallyEncoded: ")", plainText: ")"},
	{fullyEncoded: "%2A", minimallyEncoded: "*", plainText: "*"},
	{fullyEncoded: "%2B", minimallyEncoded: "%2B", plainText: "+"},
	{fullyEncoded: "%2C", minimallyEncoded: "%2C", plainText: ","},
	{fullyEncoded: "%2D", minimallyEncoded: "-", plainText: "-"},
	{fullyEncoded: "%2E", minimallyEncoded: ".", plainText: "."},
	{fullyEncoded: "%2F", minimallyEncoded: "/", plainText: "/"},
	{fullyEncoded: "%30", minimallyEncoded: "0", plainText: "0"},
	{fullyEncoded: "%31", minimallyEncoded: "1", plainText: "1"},
	{fullyEncoded: "%32", minimallyEncoded: "2", plainText: "2"},
	{fullyEncoded: "%33", minimallyEncoded: "3", plainText: "3"},
	{fullyEncoded: "%34", minimallyEncoded: "4", plainText: "4"},
	{fullyEncoded: "%35", minimallyEncoded: "5", plainText: "5"},
	{fullyEncoded: "%36", minimallyEncoded: "6", plainText: "6"},
	{fullyEncoded: "%37", minimallyEncoded: "7", plainText: "7"},
	{fullyEncoded: "%38", minimallyEncoded: "8", plainText: "8"},
	{fullyEncoded: "%39", minimallyEncoded: "9", plainText: "9"},
	{fullyEncoded: "%3A", minimallyEncoded: ":", plainText: ":"},
	{fullyEncoded: "%3B", minimallyEncoded: ";", plainText: ";"},
	{fullyEncoded: "%3C", minimallyEncoded: "%3C", plainText: "<"},
	{fullyEncoded: "%3D", minimallyEncoded: "=", plainText: "="},
	{fullyEncoded: "%3E", minimallyEncoded: "%3E", plainText: ">"},
	{fullyEncoded: "%3F", minimallyEncoded: "%3F", plainText: "?"},
	{fullyEncoded: "%40", minimallyEncoded: "@", plainText: "@"},
	{fullyEncoded: "%41", minimallyEncoded: "A", plainText: "A"},
	{fullyEncoded: "%42", minimallyEncoded: "B", plainText: "B"},
	{fullyEncoded: "%43", minimallyEncoded: "C", plainText: "C"},
	{fullyEncoded: "%44", minimallyEncoded: "D", plainText: "D"},
	{fullyEncoded: "%45", minimallyEncoded: "E", plainText: "E"},
	{fullyEncoded: "%46", minimallyEncoded: "F", plainText: "F"},
	{fullyEncoded: "%47", minimallyEncoded: "G", plainText: "G"},
	{fullyEncoded: "%48", minimallyEncoded: "H", plainText: "H"},
	{fullyEncoded: "%49", minimallyEncoded: "I", plainText: "I"},
	{fullyEncoded: "%4A", minimallyEncoded: "J", plainText: "J"},
	{fullyEncoded: "%4B", minimallyEncoded: "K", plainText: "K"},
	{fullyEncoded: "%4C", minimallyEncoded: "L", plainText: "L"},
	{fullyEncoded: "%4D", minimallyEncoded: "M", plainText: "M"},
	{fullyEncoded: "%4E", minimallyEncoded: "N", plainText: "N"},
	{fullyEncoded: "%4F", minimallyEncoded: "O", plainText: "O"},
	{fullyEncoded: "%50", minimallyEncoded: "P", plainText: "P"},
	{fullyEncoded: "%51", minimallyEncoded: "Q", plainText: "Q"},
	{fullyEncoded: "%52", minimallyEncoded: "R", plainText: "R"},
	{fullyEncoded: "%53", minimallyEncoded: "S", plainText: "S"},
	{fullyEncoded: "%54", minimallyEncoded: "T", plainText: "T"},
	{fullyEncoded: "%55", minimallyEncoded: "U", plainText: "U"},
	{fullyEncoded: "%56", minimallyEncoded: "V", plainText: "V"},
	{fullyEncoded: "%57", minimallyEncoded: "W", plainText: "W"},
	{fullyEncoded: "%58", minimallyEncoded: "X", plainText: "X"},
	{fullyEncoded: "%59", minimallyEncoded: "Y", plainText: "Y"},
	{fullyEncoded: "%5A", minimallyEncoded: "Z", plainText: "Z"},
	{fullyEncoded: "%5B", minimallyEncoded: "%5B", plainText: "["},
	{fullyEncoded: "%5C", minimallyEncoded: "%5C", plainText: "\\"},
	{fullyEncoded: "%5D", minimallyEncoded: "%5D", plainText: "]"},
	{fullyEncoded: "%5E", minimallyEncoded: "%5E", plainText: "^"},
	{fullyEncoded: "%5F", minimallyEncoded: "_", plainText: "_"},
	{fullyEncoded: "%60", minimallyEncoded: "%60", plainText: "`"},
	{fullyEncoded: "%61", minimallyEncoded: "a", plainText: "a"},
	{fullyEncoded: "%62", minimallyEncoded: "b", plainText: "b"},
	{fullyEncoded: "%63", minimallyEncoded: "c", plainText: "c"},
	{fullyEncoded: "%64", minimallyEncoded: "d", plainText: "d"},
	{fullyEncoded: "%65", minimallyEncoded: "e", plainText: "e"},
	{fullyEncoded: "%66", minimallyEncoded: "f", plainText: "f"},
	{fullyEncoded: "%67", minimallyEncoded: "g", plainText: "g"},
	{fullyEncoded: "%68", minimallyEncoded: "h", plainText: "h"},
	{fullyEncoded: "%69", minimallyEncoded: "i", plainText: "i"},
	{fullyEncoded: "%6A", minimallyEncoded: "j", plainText: "j"},
	{fullyEncoded: "%6B", minimallyEncoded: "k", plainText: "k"},
	{fullyEncoded: "%6C", minimallyEncoded: "l", plainText: "l"},
	{fullyEncoded: "%6D", minimallyEncoded: "m", plainText: "m"},
	{fullyEncoded: "%6E", minimallyEncoded: "n", plainText: "n"},
	{fullyEncoded: "%6F", minimallyEncoded: "o", plainText: "o"},
	{fullyEncoded: "%70", minimallyEncoded: "p", plainText: "p"},
	{fullyEncoded: "%71", minimallyEncoded: "q", plainText: "q"},
	{fullyEncoded: "%72", minimallyEncoded: "r", plainText: "r"},
	{fullyEncoded: "%73", minimallyEncoded: "s", plainText: "s"},
	{fullyEncoded: "%74", minimallyEncoded: "t", plainText: "t"},
	{fullyEncoded: "%75", minimallyEncoded: "u", plainText: "u"},
	{fullyEncoded: "%76", minimallyEncoded: "v", plainText: "v"},
	{fullyEncoded: "%77", minimallyEncoded: "w", plainText: "w"},
	{fullyEncoded: "%78", minimallyEncoded: "x", plainText: "x"},
	{fullyEncoded: "%79", minimallyEncoded: "y", plainText: "y"},
	{fullyEncoded: "%7A", minimallyEncoded: "z", plainText: "z"},
	{fullyEncoded: "%7B", minimallyEncoded: "%7B", plainText: "{"},
	{fullyEncoded: "%7C", minimallyEncoded: "%7C", plainText: "|"},
	{fullyEncoded: "%7D", minimallyEncoded: "%7D", plainText: "}"},
	{fullyEncoded: "%7E", minimallyEncoded: "~", plainText: "~"},
	{fullyEncoded: "%7F", minimallyEncoded: "%7F", plainText: "\u007f"},
	{fullyEncoded: "%E8%87%AA%E7%94%B1", minimallyEncoded: "%E8%87%AA%E7%94%B1", plainText: "自由"},
	{fullyEncoded: "%F0%90%90%80", minimallyEncoded: "%F0%90%90%80", plainText: "𐐀"},
}

func TestUrlEncode(t *testing.T) {
	for _, test := range encodeTest {
		got := urlEncode(test.plainText)
		if got != test.minimallyEncoded && got != test.fullyEncoded {
			t.Errorf("urlEncode(%q) got %q wanted %q or %q", test.plainText, got, test.minimallyEncoded, test.fullyEncoded)
		}
	}
}

func TestTimeString(t *testing.T) {
	for _, test := range []struct {
		in   time.Time
		want string
	}{
		{fstest.Time("1970-01-01T00:00:00.000000000Z"), "0"},
		{fstest.Time("2001-02-03T04:05:10.123123123Z"), "981173110123"},
		{fstest.Time("2001-02-03T05:05:10.123123123+01:00"), "981173110123"},
	} {
		got := timeString(test.in)
		if test.want != got {
			t.Logf("%v: want %v got %v", test.in, test.want, got)
		}
	}

}

func TestParseTimeString(t *testing.T) {
	for _, test := range []struct {
		in        string
		want      time.Time
		wantError string
	}{
		{"0", fstest.Time("1970-01-01T00:00:00.000000000Z"), ""},
		{"981173110123", fstest.Time("2001-02-03T04:05:10.123000000Z"), ""},
		{"", time.Time{}, ""},
		{"potato", time.Time{}, `strconv.ParseInt: parsing "potato": invalid syntax`},
	} {
		o := Object{}
		err := o.parseTimeString(test.in)
		got := o.modTime
		var gotError string
		if err != nil {
			gotError = err.Error()
		}
		if test.want != got {
			t.Logf("%v: want %v got %v", test.in, test.want, got)
		}
		if test.wantError != gotError {
			t.Logf("%v: want error %v got error %v", test.in, test.wantError, gotError)
		}
	}

}

// The integration tests do a reasonable job of testing the normal
// copy but don't test the chunked copy.
func (f *Fs) InternalTestChunkedCopy(t *testing.T) {
	ctx := context.Background()

	contents := random.String(8 * 1024 * 1024)
	item := fstest.NewItem("chunked-copy", contents, fstest.Time("2001-05-06T04:05:06.499999999Z"))
	src := fstests.PutTestContents(ctx, t, f, &item, contents, true)
	defer func() {
		assert.NoError(t, src.Remove(ctx))
	}()

	var itemCopy = item
	itemCopy.Path += ".copy"

	// Set copy cutoff to mininum value so we make chunks
	origCutoff := f.opt.CopyCutoff
	f.opt.CopyCutoff = minChunkSize
	defer func() {
		f.opt.CopyCutoff = origCutoff
	}()

	// Do the copy
	dst, err := f.Copy(ctx, src, itemCopy.Path)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, dst.Remove(ctx))
	}()

	// Check size
	assert.Equal(t, src.Size(), dst.Size())

	// Check modtime
	srcModTime := src.ModTime(ctx)
	dstModTime := dst.ModTime(ctx)
	assert.True(t, srcModTime.Equal(dstModTime))

	// Make sure contents are correct
	gotContents := fstests.ReadObject(ctx, t, dst, -1)
	assert.Equal(t, contents, gotContents)
}

// The integration tests do a reasonable job of testing the normal
// streaming upload but don't test the chunked streaming upload.
func (f *Fs) InternalTestChunkedStreamingUpload(t *testing.T, size int) {
	ctx := context.Background()
	contents := random.String(size)
	item := fstest.NewItem(fmt.Sprintf("chunked-streaming-upload-%d", size), contents, fstest.Time("2001-05-06T04:05:06.499Z"))

	// Set chunk size to mininum value so we make chunks
	origOpt := f.opt
	f.opt.ChunkSize = minChunkSize
	f.opt.UploadCutoff = 0
	defer func() {
		f.opt = origOpt
	}()

	// Do the streaming upload
	src := object.NewStaticObjectInfo(item.Path, item.ModTime, -1, true, item.Hashes, f)
	in := bytes.NewBufferString(contents)
	dst, err := f.PutStream(ctx, in, src)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, dst.Remove(ctx))
	}()

	// Check size
	assert.Equal(t, int64(size), dst.Size())

	// Check modtime
	srcModTime := src.ModTime(ctx)
	dstModTime := dst.ModTime(ctx)
	assert.Equal(t, srcModTime, dstModTime)

	// Make sure contents are correct
	gotContents := fstests.ReadObject(ctx, t, dst, -1)
	assert.Equal(t, contents, gotContents, "Contents incorrect")
}

// -run TestIntegration/FsMkdir/FsPutFiles/Internal
func (f *Fs) InternalTest(t *testing.T) {
	t.Run("ChunkedCopy", f.InternalTestChunkedCopy)
	for _, size := range []fs.SizeSuffix{
		minChunkSize - 1,
		minChunkSize,
		minChunkSize + 1,
		(3 * minChunkSize) / 2,
		(5 * minChunkSize) / 2,
	} {
		t.Run(fmt.Sprintf("ChunkedStreamingUpload/%d", size), func(t *testing.T) {
			f.InternalTestChunkedStreamingUpload(t, int(size))
		})
	}
}

var _ fstests.InternalTester = (*Fs)(nil)
