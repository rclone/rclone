// Serve s3 tests set up a server and run the integration tests
// for the s3 remote against it.

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rclone/rclone/fs/object"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/proxy/proxyflags"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	httplib "github.com/rclone/rclone/lib/http"
	"github.com/rclone/rclone/lib/random"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	endpoint = "localhost:0"
)

// Configure and serve the server
func serveS3(f fs.Fs) (testURL string, keyid string, keysec string, w *Server) {
	keyid = random.String(16)
	keysec = random.String(16)
	serveropt := &Options{
		HTTP:           httplib.DefaultCfg(),
		pathBucketMode: true,
		hashName:       "",
		hashType:       hash.None,
		authPair:       []string{fmt.Sprintf("%s,%s", keyid, keysec)},
	}

	serveropt.HTTP.ListenAddr = []string{endpoint}
	w, _ = newServer(context.Background(), f, serveropt)
	router := w.server.Router()

	w.Bind(router)
	_ = w.Serve()
	testURL = w.server.URLs()[0]

	return
}

// TestS3 runs the s3 server then runs the unit tests for the
// s3 remote against it.
func TestS3(t *testing.T) {
	start := func(f fs.Fs) (configmap.Simple, func()) {
		testURL, keyid, keysec, _ := serveS3(f)
		// Config for the backend we'll use to connect to the server
		config := configmap.Simple{
			"type":              "s3",
			"provider":          "Rclone",
			"endpoint":          testURL,
			"access_key_id":     keyid,
			"secret_access_key": keysec,
		}

		return config, func() {}
	}

	servetest.Run(t, "s3", start)
}

// tests using the minio client
func TestEncodingWithMinioClient(t *testing.T) {
	cases := []struct {
		description string
		bucket      string
		path        string
		filename    string
		expected    string
	}{
		{
			description: "weird file in bucket root",
			bucket:      "mybucket",
			path:        "",
			filename:    " file with w€r^d ch@r \\#~+§4%&'. txt ",
		},
		{
			description: "weird file inside a weird folder",
			bucket:      "mybucket",
			path:        "ä#/नेपाल&/?/",
			filename:    " file with w€r^d ch@r \\#~+§4%&'. txt ",
		},
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			fstest.Initialise()
			f, _, clean, err := fstest.RandomRemote()
			assert.NoError(t, err)
			defer clean()
			err = f.Mkdir(context.Background(), path.Join(tt.bucket, tt.path))
			assert.NoError(t, err)

			buf := bytes.NewBufferString("contents")
			uploadHash := hash.NewMultiHasher()
			in := io.TeeReader(buf, uploadHash)

			obji := object.NewStaticObjectInfo(
				path.Join(tt.bucket, tt.path, tt.filename),
				time.Now(),
				int64(buf.Len()),
				true,
				nil,
				nil,
			)
			_, err = f.Put(context.Background(), in, obji)
			assert.NoError(t, err)

			endpoint, keyid, keysec, _ := serveS3(f)
			testURL, _ := url.Parse(endpoint)
			minioClient, err := minio.New(testURL.Host, &minio.Options{
				Creds:  credentials.NewStaticV4(keyid, keysec, ""),
				Secure: false,
			})
			assert.NoError(t, err)

			buckets, err := minioClient.ListBuckets(context.Background())
			assert.NoError(t, err)
			assert.Equal(t, buckets[0].Name, tt.bucket)
			objects := minioClient.ListObjects(context.Background(), tt.bucket, minio.ListObjectsOptions{
				Recursive: true,
			})
			for object := range objects {
				assert.Equal(t, path.Join(tt.path, tt.filename), object.Key)
			}
		})
	}
}

type FileStuct struct {
	path     string
	filename string
}

type TestCase struct {
	description string
	bucket      string
	files       []FileStuct
	keyID       string
	keySec      string
	shouldFail  bool
}

func testListBuckets(t *testing.T, cases []TestCase, useProxy bool) {
	fstest.Initialise()

	var f fs.Fs
	if useProxy {
		// the backend config will be made by the proxy
		prog, err := filepath.Abs("../servetest/proxy_code.go")
		require.NoError(t, err)
		files, err := filepath.Abs("testdata")
		require.NoError(t, err)
		cmd := "go run " + prog + " " + files

		// FIXME: this is untidy setting a global variable!
		proxyflags.Opt.AuthProxy = cmd
		defer func() {
			proxyflags.Opt.AuthProxy = ""
		}()

		f = nil
	} else {
		// create a test Fs
		var err error
		f, err = fs.NewFs(context.Background(), "testdata")
		require.NoError(t, err)
	}

	for _, tt := range cases {
		t.Run(tt.description, func(t *testing.T) {
			endpoint, keyid, keysec, s := serveS3(f)
			defer func() {
				assert.NoError(t, s.server.Shutdown())
			}()

			if tt.keyID != "" {
				keyid = tt.keyID
			}
			if tt.keySec != "" {
				keysec = tt.keySec
			}

			testURL, _ := url.Parse(endpoint)
			minioClient, err := minio.New(testURL.Host, &minio.Options{
				Creds:  credentials.NewStaticV4(keyid, keysec, ""),
				Secure: false,
			})
			assert.NoError(t, err)

			buckets, err := minioClient.ListBuckets(context.Background())
			if tt.shouldFail {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.NotEmpty(t, buckets)
				assert.Equal(t, buckets[0].Name, tt.bucket)

				o := minioClient.ListObjects(context.Background(), tt.bucket, minio.ListObjectsOptions{
					Recursive: true,
				})
				// save files after reading from channel
				objects := []string{}
				for object := range o {
					objects = append(objects, object.Key)
				}

				for _, tt := range tt.files {
					file := path.Join(tt.path, tt.filename)
					found := false
					for _, fname := range objects {
						if file == fname {
							found = true
							break
						}
					}
					require.Equal(t, true, found, "Object not found: "+file)
				}
			}
		})
	}
}

func TestListBuckets(t *testing.T) {
	var cases = []TestCase{
		{
			description: "list buckets",
			bucket:      "mybucket",
			files: []FileStuct{
				{
					path:     "",
					filename: "lorem.txt",
				},
				{
					path:     "foo",
					filename: "bar.txt",
				},
			},
		},
		{
			description: "list buckets: wrong s3 key",
			bucket:      "mybucket",
			keyID:       "invalid",
			shouldFail:  true,
		},
		{
			description: "list buckets: wrong s3 secret",
			bucket:      "mybucket",
			keySec:      "invalid",
			shouldFail:  true,
		},
	}

	testListBuckets(t, cases, false)
}

func TestListBucketsAuthProxy(t *testing.T) {
	var cases = []TestCase{
		{
			description: "list buckets",
			bucket:      "mybucket",
			// request with random keyid
			// instead of what was set in 'authPair'
			keyID: random.String(16),
			files: []FileStuct{
				{
					path:     "",
					filename: "lorem.txt",
				},
				{
					path:     "foo",
					filename: "bar.txt",
				},
			},
		},
		{
			description: "list buckets: wrong s3 secret",
			bucket:      "mybucket",
			keySec:      "invalid",
			shouldFail:  true,
		},
	}

	testListBuckets(t, cases, true)
}
