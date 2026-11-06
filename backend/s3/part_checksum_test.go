package s3

import (
	"bufio"
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"hash/crc32"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeMultipartServer is just enough of S3 to do an upload. It checks
// the checksums like S3 does and records what it got.
type fakeMultipartServer struct {
	t  *testing.T
	mu sync.Mutex
	// if set, UploadPart does not return the CRC32C of the part
	noCRC32CResponse bool
	// if set, the full object CRC32C differs from the parts
	badFullCRC32C bool
	// if set, CompleteMultipartUpload does not return the CRC32C
	noFullCRC32CResponse bool
	// from CreateMultipartUpload
	checksumAlgorithm string
	checksumType      string
	// from CompleteMultipartUpload
	completeChecksumType string
	completeCRC32C       string
	// the CRC32C the object stores
	objectCRC32C string
	// per part number
	parts      map[int][]byte
	contentMD5 map[int]string
	crc32c     map[int]string
	// from CompleteMultipartUpload
	completed []fakeCompletedPart
	// parts sent with aws-chunked encoding and a trailing checksum
	trailers int
	// unexpected requests
	unexpected []string
}

type fakeCompletedPart struct {
	PartNumber     int
	ETag           string
	ChecksumCRC32C string
}

func newFakeMultipartServer(t *testing.T) *fakeMultipartServer {
	return &fakeMultipartServer{
		t:          t,
		parts:      map[int][]byte{},
		contentMD5: map[int]string{},
		crc32c:     map[int]string{},
	}
}

var crc32cTable = crc32.MakeTable(crc32.Castagnoli)

func crc32cBase64(data []byte) string {
	var sum [4]byte
	binary.BigEndian.PutUint32(sum[:], crc32.Checksum(data, crc32cTable))
	return base64.StdEncoding.EncodeToString(sum[:])
}

func md5Base64(data []byte) string {
	sum := md5.Sum(data)
	return base64.StdEncoding.EncodeToString(sum[:])
}

func badDigest(w http.ResponseWriter) {
	w.WriteHeader(http.StatusBadRequest)
	_, _ = io.WriteString(w, `<Error><Code>BadDigest</Code></Error>`)
}

func (s *fakeMultipartServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q := r.URL.Query()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		// rclone cancels the other parts when an upload fails
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	switch {
	case r.Method == http.MethodPost && q.Has("uploads"):
		s.checksumAlgorithm = r.Header.Get("X-Amz-Checksum-Algorithm")
		s.checksumType = r.Header.Get("X-Amz-Checksum-Type")
		_, _ = io.WriteString(w, `<InitiateMultipartUploadResult><Bucket>bucket</Bucket><Key>file</Key><UploadId>upload-1</UploadId></InitiateMultipartUploadResult>`)
	case r.Method == http.MethodPut:
		part := 0 // PutObject
		if q.Has("partNumber") {
			part, err = strconv.Atoi(q.Get("partNumber"))
			if !assert.NoError(s.t, err) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if strings.Contains(r.Header.Get("Content-Encoding"), "aws-chunked") {
			var trailer map[string]string
			body, trailer, err = decodeAWSChunked(body)
			if !assert.NoError(s.t, err) {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			for k, v := range trailer {
				r.Header.Set(k, v)
			}
			s.trailers++
		}
		if sum := r.Header.Get("Content-Md5"); sum != "" {
			s.contentMD5[part] = sum
			if sum != md5Base64(body) {
				badDigest(w)
				return
			}
		}
		if sum := r.Header.Get("X-Amz-Checksum-Crc32c"); sum != "" {
			s.crc32c[part] = sum
			if sum != crc32cBase64(body) {
				badDigest(w)
				return
			}
			if !s.noCRC32CResponse {
				w.Header().Set("X-Amz-Checksum-Crc32c", sum)
			}
			if part == 0 {
				s.objectCRC32C = sum
			}
		}
		s.parts[part] = body
		sum := md5.Sum(body)
		w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:])+`"`)
	case r.Method == http.MethodPost && q.Has("uploadId"):
		var req struct {
			Parts []fakeCompletedPart `xml:"Part"`
		}
		if !assert.NoError(s.t, xml.Unmarshal(body, &req)) {
			http.Error(w, "bad CompleteMultipartUpload", http.StatusBadRequest)
			return
		}
		s.completed = req.Parts
		s.completeChecksumType = r.Header.Get("X-Amz-Checksum-Type")
		s.completeCRC32C = r.Header.Get("X-Amz-Checksum-Crc32c")
		var md5s, object []byte
		for i := 1; i <= len(s.parts); i++ {
			sum := md5.Sum(s.parts[i])
			md5s = append(md5s, sum[:]...)
			object = append(object, s.parts[i]...)
		}
		// S3 checks a full object CRC32C against the parts
		if s.checksumType == "FULL_OBJECT" {
			if s.badFullCRC32C {
				object = append(object, 0)
			}
			s.objectCRC32C = crc32cBase64(object)
		}
		if s.completeCRC32C != "" && (s.completeChecksumType != s.checksumType || s.completeCRC32C != s.objectCRC32C) {
			badDigest(w)
			return
		}
		response := s.objectCRC32C
		if s.noFullCRC32CResponse {
			response = ""
		}
		etag := md5.Sum(md5s)
		_, _ = fmt.Fprintf(w, `<CompleteMultipartUploadResult><Bucket>bucket</Bucket><Key>file</Key><ETag>"%s-%d"</ETag><ChecksumCRC32C>%s</ChecksumCRC32C></CompleteMultipartUploadResult>`, hex.EncodeToString(etag[:]), len(s.parts), response)
	case r.Method == http.MethodHead:
		var object []byte
		for i := range len(s.parts) + 1 {
			object = append(object, s.parts[i]...)
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(object)))
	case r.Method == http.MethodDelete && q.Has("uploadId"):
		w.WriteHeader(http.StatusNoContent) // AbortMultipartUpload
	default:
		s.unexpected = append(s.unexpected, r.Method+" "+r.URL.String())
		w.WriteHeader(http.StatusNotImplemented)
	}
}

// decodeAWSChunked decodes an unsigned aws-chunked body: chunks of
// "<hex size>\r\n<data>\r\n", a chunk of size 0, then trailer lines
func decodeAWSChunked(body []byte) (data []byte, trailer map[string]string, err error) {
	r := bufio.NewReader(bytes.NewReader(body))
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return nil, nil, err
		}
		size, err := strconv.ParseInt(strings.TrimSpace(strings.SplitN(line, ";", 2)[0]), 16, 64)
		if err != nil {
			return nil, nil, fmt.Errorf("chunk size %q: %w", line, err)
		}
		if size == 0 {
			break
		}
		chunk := make([]byte, size+2)
		if _, err = io.ReadFull(r, chunk); err != nil {
			return nil, nil, err
		}
		data = append(data, chunk[:size]...)
	}
	trailer = map[string]string{}
	for {
		line, err := r.ReadString('\n')
		line = strings.TrimSpace(line)
		if line == "" || err != nil {
			break
		}
		k, v, ok := strings.Cut(line, ":")
		if !ok {
			return nil, nil, fmt.Errorf("trailer %q", line)
		}
		trailer[k] = v
	}
	return data, trailer, nil
}

// uploadWithPartChecksum uploads data in 5 MiB parts to fake, over
// https if useTLS. Data under 5 MiB is a single part upload unless
// stream is set.
func uploadWithPartChecksum(t *testing.T, fake *fakeMultipartServer, checksum string, useTLS bool, data []byte, stream bool) error {
	ctx := context.Background()
	srv := httptest.NewUnstartedServer(fake)
	if useTLS {
		srv.StartTLS()
		var ci *fs.ConfigInfo
		ctx, ci = fs.AddConfig(ctx)
		ci.InsecureSkipVerify = true
	} else {
		srv.Start()
	}
	defer srv.Close()

	f, err := fs.NewFs(ctx, fmt.Sprintf(":s3,provider=Other,endpoint='%s',access_key_id=key,secret_access_key=secret,force_path_style,no_check_bucket,no_head,chunk_size=5Mi,upload_cutoff=5Mi,upload_concurrency=2,part_checksum=%s:bucket", srv.URL, checksum))
	require.NoError(t, err)

	size := int64(len(data))
	if stream {
		size = -1
	}
	src := object.NewStaticObjectInfo("file", time.Now(), size, true, nil, nil)
	_, err = f.Put(ctx, bytes.NewReader(data), src)
	return err
}

func TestPartChecksum(t *testing.T) {
	data := []byte(random.String(12 << 20))
	partData := func(i int) []byte {
		start := (i - 1) * 5 << 20
		return data[start:min(start+5<<20, len(data))]
	}

	for _, test := range []struct {
		checksum     string
		useTLS       bool
		stream       bool
		wantAlgo     string // on CreateMultipartUpload
		wantMD5      bool   // Content-MD5 on the parts
		wantCRC32C   bool   // CRC32C on the parts and of the object
		wantTrailers int    // parts sent with a trailing checksum
	}{
		{checksum: "md5", wantMD5: true},
		{checksum: "md5", useTLS: true, wantMD5: true},
		{checksum: "crc32c", wantAlgo: "CRC32C", wantCRC32C: true},
		{checksum: "crc32c", useTLS: true, wantAlgo: "CRC32C", wantCRC32C: true, wantTrailers: 3},
		{checksum: "crc32c", useTLS: true, stream: true, wantAlgo: "CRC32C", wantCRC32C: true, wantTrailers: 3},
		{checksum: "none"},
		{checksum: "none", useTLS: true},
	} {
		t.Run(fmt.Sprintf("%s,tls=%v,stream=%v", test.checksum, test.useTLS, test.stream), func(t *testing.T) {
			fake := newFakeMultipartServer(t)
			require.NoError(t, uploadWithPartChecksum(t, fake, test.checksum, test.useTLS, data, test.stream))
			require.Empty(t, fake.unexpected)
			require.Len(t, fake.parts, 3)
			require.Len(t, fake.completed, 3)
			var got []byte
			for i := 1; i <= 3; i++ {
				got = append(got, fake.parts[i]...)
			}
			require.Equal(t, data, got, "the parts make up the file")

			assert.Equal(t, test.wantAlgo, fake.checksumAlgorithm)
			assert.Equal(t, test.wantTrailers, fake.trailers)
			wantType, wantFull := "", ""
			if test.wantCRC32C {
				wantType, wantFull = "FULL_OBJECT", crc32cBase64(data)
			}
			assert.Equal(t, wantType, fake.checksumType, "checksum type on create")
			assert.Equal(t, wantType, fake.completeChecksumType, "checksum type on complete")
			assert.Equal(t, wantFull, fake.completeCRC32C, "CRC32C of the object on complete")
			assert.Equal(t, wantFull, fake.objectCRC32C, "CRC32C stored with the object")
			for i := 1; i <= 3; i++ {
				wantMD5, wantCRC32C := "", ""
				if test.wantMD5 {
					wantMD5 = md5Base64(partData(i))
				}
				if test.wantCRC32C {
					wantCRC32C = crc32cBase64(partData(i))
				}
				assert.Equal(t, wantMD5, fake.contentMD5[i], "Content-MD5 of part %d", i)
				assert.Equal(t, wantCRC32C, fake.crc32c[i], "CRC32C of part %d", i)
			}
			for _, p := range fake.completed {
				wantCRC32C := ""
				if test.wantCRC32C {
					wantCRC32C = crc32cBase64(partData(p.PartNumber))
				}
				assert.Equal(t, wantCRC32C, p.ChecksumCRC32C, "CRC32C of completed part %d", p.PartNumber)
			}
		})
	}

	for _, checksum := range []string{"md5", "crc32c", "none"} {
		for _, size := range []int{0, 1000} {
			for _, useTLS := range []bool{false, true} {
				t.Run(fmt.Sprintf("single,%s,size=%d,tls=%v", checksum, size, useTLS), func(t *testing.T) {
					fake := newFakeMultipartServer(t)
					require.NoError(t, uploadWithPartChecksum(t, fake, checksum, useTLS, data[:size], false))
					require.Empty(t, fake.unexpected)
					assert.Equal(t, data[:size], fake.parts[0])
					want := ""
					if checksum == "crc32c" && useTLS { // over http the body can't seek
						want = crc32cBase64(data[:size])
					}
					assert.Equal(t, want, fake.objectCRC32C, "CRC32C stored with the object")
				})
			}
		}
	}

	t.Run("crc32c,stream,empty", func(t *testing.T) {
		fake := newFakeMultipartServer(t)
		require.NoError(t, uploadWithPartChecksum(t, fake, "crc32c", true, nil, true))
		require.Empty(t, fake.unexpected)
		require.Len(t, fake.completed, 1)
		assert.Equal(t, "AAAAAA==", fake.objectCRC32C)
	})

	for _, test := range []struct {
		name    string
		setup   func(*fakeMultipartServer)
		wantErr string
	}{{
		name:    "crc32c not returned",
		setup:   func(s *fakeMultipartServer) { s.noCRC32CResponse = true },
		wantErr: "has no CRC32C: the provider may not support --s3-part-checksum crc32c",
	}, {
		name:    "wrong object crc32c",
		setup:   func(s *fakeMultipartServer) { s.badFullCRC32C = true },
		wantErr: "BadDigest",
	}, {
		name:    "object crc32c not returned",
		setup:   func(s *fakeMultipartServer) { s.noFullCRC32CResponse = true },
		wantErr: "has no CRC32C: the provider may not support --s3-part-checksum crc32c",
	}} {
		t.Run(test.name, func(t *testing.T) {
			fake := newFakeMultipartServer(t)
			test.setup(fake)
			err := uploadWithPartChecksum(t, fake, "crc32c", false, data, false)
			require.Error(t, err)
			assert.Contains(t, err.Error(), test.wantErr)
		})
	}
}

func TestCRC32CCombine(t *testing.T) {
	data := []byte(random.String(1<<20 + 12345))
	for _, cuts := range [][]int{{}, {0}, {1}, {0, 0, 7}, {5, 5, 6, 1 << 20}, {len(data)}} {
		var crc uint32
		prev := 0
		for _, cut := range append(cuts, len(data)) {
			crc = crc32cCombine(crc, crc32.Checksum(data[prev:cut], crc32cTable), int64(cut-prev))
			prev = cut
		}
		assert.Equal(t, crc32.Checksum(data, crc32cTable), crc, "cuts %v", cuts)
	}
	assert.Equal(t, uint32(0), crc32cCombine(0, 0, 0), "zero length object")
}
