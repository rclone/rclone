// Copyright 2011 Google Inc. All rights reserved.
// Use of this source code is governed by the Apache 2.0
// license that can be found in the LICENSE file.

package blobstore

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"mime/quotedprintable"
	"net/http"
	"net/textproto"
	"os"
	"strconv"
	"strings"
	"testing"

	"golang.org/x/text/encoding/htmlindex"

	"google.golang.org/appengine"
	"google.golang.org/appengine/internal/aetesting"

	pb "google.golang.org/appengine/internal/blobstore"
)

const rbs = readBufferSize

const charsetUTF8 = "utf-8"
const charsetISO2022JP = "iso-2022-jp"
const nonASCIIStr = "Hello, 世界"

func min(x, y int) int {
	if x < y {
		return x
	}
	return y
}

func fakeFetchData(req *pb.FetchDataRequest, res *pb.FetchDataResponse) error {
	i0 := int(*req.StartIndex)
	i1 := int(*req.EndIndex + 1) // Blobstore's end-indices are inclusive; Go's are exclusive.
	bk := *req.BlobKey
	if i := strings.Index(bk, "."); i != -1 {
		// Strip everything past the ".".
		bk = bk[:i]
	}
	switch bk {
	case "a14p":
		const s = "abcdefghijklmnop"
		i0 := min(len(s), i0)
		i1 := min(len(s), i1)
		res.Data = []byte(s[i0:i1])
	case "longBlob":
		res.Data = make([]byte, i1-i0)
		for i := range res.Data {
			res.Data[i] = 'A' + uint8(i0/rbs)
			i0++
		}
	}
	return nil
}

// step is one step of a readerTest.
// It consists of a Reader method to call, the method arguments
// (lenp, offset, whence) and the expected results.
type step struct {
	method  string
	lenp    int
	offset  int64
	whence  int
	want    string
	wantErr error
}

var readerTest = []struct {
	blobKey string
	step    []step
}{
	{"noSuchBlobKey", []step{
		{"Read", 8, 0, 0, "", io.EOF},
	}},
	{"a14p.0", []step{
		// Test basic reads.
		{"Read", 1, 0, 0, "a", nil},
		{"Read", 3, 0, 0, "bcd", nil},
		{"Read", 1, 0, 0, "e", nil},
		{"Read", 2, 0, 0, "fg", nil},
		// Test Seek.
		{"Seek", 0, 2, os.SEEK_SET, "2", nil},
		{"Read", 5, 0, 0, "cdefg", nil},
		{"Seek", 0, 2, os.SEEK_CUR, "9", nil},
		{"Read", 1, 0, 0, "j", nil},
		// Test reads up to and past EOF.
		{"Read", 5, 0, 0, "klmno", nil},
		{"Read", 5, 0, 0, "p", nil},
		{"Read", 5, 0, 0, "", io.EOF},
		// Test ReadAt.
		{"ReadAt", 4, 0, 0, "abcd", nil},
		{"ReadAt", 4, 3, 0, "defg", nil},
		{"ReadAt", 4, 12, 0, "mnop", nil},
		{"ReadAt", 4, 13, 0, "nop", io.EOF},
		{"ReadAt", 4, 99, 0, "", io.EOF},
	}},
	{"a14p.1", []step{
		// Test Seek before any reads.
		{"Seek", 0, 2, os.SEEK_SET, "2", nil},
		{"Read", 1, 0, 0, "c", nil},
		// Test that ReadAt doesn't affect the Read offset.
		{"ReadAt", 3, 9, 0, "jkl", nil},
		{"Read", 3, 0, 0, "def", nil},
	}},
	{"a14p.2", []step{
		// Test ReadAt before any reads or seeks.
		{"ReadAt", 2, 14, 0, "op", nil},
	}},
	{"longBlob.0", []step{
		// Test basic read.
		{"Read", 1, 0, 0, "A", nil},
		// Test that Read returns early when the buffer is exhausted.
		{"Seek", 0, rbs - 2, os.SEEK_SET, strconv.Itoa(rbs - 2), nil},
		{"Read", 5, 0, 0, "AA", nil},
		{"Read", 3, 0, 0, "BBB", nil},
		// Test that what we just read is still in the buffer.
		{"Seek", 0, rbs - 2, os.SEEK_SET, strconv.Itoa(rbs - 2), nil},
		{"Read", 5, 0, 0, "AABBB", nil},
		// Test ReadAt.
		{"ReadAt", 3, rbs - 4, 0, "AAA", nil},
		{"ReadAt", 6, rbs - 4, 0, "AAAABB", nil},
		{"ReadAt", 8, rbs - 4, 0, "AAAABBBB", nil},
		{"ReadAt", 5, rbs - 4, 0, "AAAAB", nil},
		{"ReadAt", 2, rbs - 4, 0, "AA", nil},
		// Test seeking backwards from the Read offset.
		{"Seek", 0, 2*rbs - 8, os.SEEK_SET, strconv.Itoa(2*rbs - 8), nil},
		{"Read", 1, 0, 0, "B", nil},
		{"Read", 1, 0, 0, "B", nil},
		{"Read", 1, 0, 0, "B", nil},
		{"Read", 1, 0, 0, "B", nil},
		{"Read", 8, 0, 0, "BBBBCCCC", nil},
	}},
	{"longBlob.1", []step{
		// Test ReadAt with a slice larger than the buffer size.
		{"LargeReadAt", 2*rbs - 2, 0, 0, strconv.Itoa(2*rbs - 2), nil},
		{"LargeReadAt", 2*rbs - 1, 0, 0, strconv.Itoa(2*rbs - 1), nil},
		{"LargeReadAt", 2*rbs + 0, 0, 0, strconv.Itoa(2*rbs + 0), nil},
		{"LargeReadAt", 2*rbs + 1, 0, 0, strconv.Itoa(2*rbs + 1), nil},
		{"LargeReadAt", 2*rbs + 2, 0, 0, strconv.Itoa(2*rbs + 2), nil},
		{"LargeReadAt", 2*rbs - 2, 1, 0, strconv.Itoa(2*rbs - 2), nil},
		{"LargeReadAt", 2*rbs - 1, 1, 0, strconv.Itoa(2*rbs - 1), nil},
		{"LargeReadAt", 2*rbs + 0, 1, 0, strconv.Itoa(2*rbs + 0), nil},
		{"LargeReadAt", 2*rbs + 1, 1, 0, strconv.Itoa(2*rbs + 1), nil},
		{"LargeReadAt", 2*rbs + 2, 1, 0, strconv.Itoa(2*rbs + 2), nil},
	}},
}

func TestReader(t *testing.T) {
	for _, rt := range readerTest {
		c := aetesting.FakeSingleContext(t, "blobstore", "FetchData", fakeFetchData)
		r := NewReader(c, appengine.BlobKey(rt.blobKey))
		for i, step := range rt.step {
			var (
				got    string
				gotErr error
				n      int
				offset int64
			)
			switch step.method {
			case "LargeReadAt":
				p := make([]byte, step.lenp)
				n, gotErr = r.ReadAt(p, step.offset)
				got = strconv.Itoa(n)
			case "Read":
				p := make([]byte, step.lenp)
				n, gotErr = r.Read(p)
				got = string(p[:n])
			case "ReadAt":
				p := make([]byte, step.lenp)
				n, gotErr = r.ReadAt(p, step.offset)
				got = string(p[:n])
			case "Seek":
				offset, gotErr = r.Seek(step.offset, step.whence)
				got = strconv.FormatInt(offset, 10)
			default:
				t.Fatalf("unknown method: %s", step.method)
			}
			if gotErr != step.wantErr {
				t.Fatalf("%s step %d: got error %v want %v", rt.blobKey, i, gotErr, step.wantErr)
			}
			if got != step.want {
				t.Fatalf("%s step %d: got %q want %q", rt.blobKey, i, got, step.want)
			}
		}
	}
}

// doPlainTextParseUploadTest tests ParseUpload's decoding of non-file form fields.
// It ensures that MIME multipart parts with Content-Type not equal to
// "message/external-body" (i.e. form fields that are not file uploads) are decoded
// correctly according to the value of their Content-Transfer-Encoding header field.
// If charset is not the empty string it will be set in the request's Content-Type
// header field, and if encoding is not the empty string then the Content-Transfer-Encoding
// header field will be set.
func doPlainTextParseUploadTest(t *testing.T, charset string, encoding string,
	rawContent string, encodedContent string) {
	bodyBuf := &bytes.Buffer{}
	w := multipart.NewWriter(bodyBuf)

	fieldName := "foo"
	hdr := textproto.MIMEHeader{}
	hdr.Set("Content-Disposition", fmt.Sprintf("form-data; name=%q", fieldName))

	if charset != "" {
		hdr.Set("Content-Type", fmt.Sprintf("text/plain; charset=%q", charset))
	} else {
		hdr.Set("Content-Type", "text/plain")
	}

	if encoding != "" {
		hdr.Set("Content-Transfer-Encoding", encoding)
	}

	pw, err := w.CreatePart(hdr)
	if err != nil {
		t.Fatalf("error creating part: %v", err)
	}
	pw.Write([]byte(encodedContent))

	if err := w.Close(); err != nil {
		t.Fatalf("error closing multipart writer: %v\n", err)
	}

	req, err := http.NewRequest("POST", "/upload", bodyBuf)
	if err != nil {
		t.Fatalf("error creating request: %v", err)
	}

	req.Header.Set("Content-Type", w.FormDataContentType())
	_, other, err := ParseUpload(req)
	if err != nil {
		t.Fatalf("error parsing upload: %v", err)
	}

	if other[fieldName][0] != rawContent {
		t.Errorf("got %q expected %q", other[fieldName][0], rawContent)
	}
}

func TestParseUploadUTF8Base64Encoding(t *testing.T) {
	encoded := base64.StdEncoding.EncodeToString([]byte(nonASCIIStr))
	doPlainTextParseUploadTest(t, charsetUTF8, "base64", nonASCIIStr, encoded)
}

func TestParseUploadUTF8Base64EncodingMultiline(t *testing.T) {
	testStr := "words words words words words words words words words words words words"
	encoded := "d29yZHMgd29yZHMgd29yZHMgd29yZHMgd29yZHMgd29yZHMgd29yZHMgd29yZHMgd29yZHMgd29y\r\nZHMgd29yZHMgd29yZHM="
	doPlainTextParseUploadTest(t, charsetUTF8, "base64", testStr, encoded)
}

func TestParseUploadUTF8QuotedPrintableEncoding(t *testing.T) {
	var encoded bytes.Buffer
	writer := quotedprintable.NewWriter(&encoded)
	writer.Write([]byte(nonASCIIStr))
	writer.Close()

	doPlainTextParseUploadTest(t, charsetUTF8, "quoted-printable", nonASCIIStr,
		encoded.String())
}

func TestParseUploadISO2022JPBase64Encoding(t *testing.T) {
	testStr := "こんにちは"
	encoding, err := htmlindex.Get(charsetISO2022JP)
	if err != nil {
		t.Fatalf("error getting encoding: %v", err)
	}

	charsetEncoded, err := encoding.NewEncoder().String(testStr)
	if err != nil {
		t.Fatalf("error encoding string: %v", err)
	}

	base64Encoded := base64.StdEncoding.EncodeToString([]byte(charsetEncoded))
	doPlainTextParseUploadTest(t, charsetISO2022JP, "base64", testStr, base64Encoded)
}

func TestParseUploadNoEncoding(t *testing.T) {
	doPlainTextParseUploadTest(t, "", "", "Hello", "Hello")
}
