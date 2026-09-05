// Serve s3 tests set up a server and run the integration tests
// for the s3 remote against it.

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rclone/gofakes3/signature"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3" // for TestS3Minio backing remote
	"github.com/rclone/rclone/cmd/serve/proxy"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/rc"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/testy"
	"github.com/rclone/rclone/lib/random"
	"github.com/rclone/rclone/vfs/vfscommon"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	endpoint = "localhost:0"
)

// Configure and serve the server
//
// If f is nil the server is expected to be using an auth proxy, in
// which case the credentials are handed to the test proxy via the
// environment rather than --auth-key.
func serveS3(t *testing.T, f fs.Fs) (testURL string, keyid string, keysec string, w *Server) {
	keyid = random.String(16)
	keysec = random.String(16)
	opt := Opt // copy default options
	if f == nil {
		t.Setenv("RCLONE_TEST_PROXY_AUTH_KEY", fmt.Sprintf("%s,%s", keyid, keysec))
	} else {
		opt.AuthKey = []string{fmt.Sprintf("%s,%s", keyid, keysec)}
	}
	opt.HTTP.ListenAddr = []string{endpoint}
	w, _ = newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxy.Opt)
	go func() {
		require.NoError(t, w.Serve())
	}()
	testURL = w.server.URLs()[0]

	return
}

// startS3 builds the start callback that brings up a serve s3 server
// wrapping f and returns the client config for connecting to it.
func startS3(t *testing.T) servetest.StartFn {
	return func(f fs.Fs) (configmap.Simple, func()) {
		testURL, keyid, keysec, _ := serveS3(t, f)
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
}

// TestS3 runs the s3 server backed by a local directory then runs the
// s3 backend integration tests against it. The local backend only
// supports OpenWriterAt, so this exercises that streaming path in
// serve s3.
func TestS3(t *testing.T) {
	servetest.Run(t, "s3", startS3(t))
}

// TestS3Minio runs the s3 server backed by a minio docker container
// (via fstest/testserver/init.d/TestS3Minio) then runs the s3 backend
// integration tests against it. Minio supports OpenChunkWriter, so
// this exercises that streaming path in serve s3 - the path the local
// backing in TestS3 cannot cover.
func TestS3Minio(t *testing.T) {
	testy.SkipUnlessDocker(t)
	servetest.RunWithBackend(t, "s3", startS3(t), "TestS3Minio:")
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

			endpoint, keyid, keysec, _ := serveS3(t, f)
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
		proxy.Opt.AuthProxy = cmd
		defer func() {
			proxy.Opt.AuthProxy = ""
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
			endpoint, keyid, keysec, s := serveS3(t, f)
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
					found := slices.Contains(objects, file)
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
			description: "list buckets: unknown s3 key",
			bucket:      "mybucket",
			keyID:       random.String(16),
			shouldFail:  true,
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

// TestNewServerPerServerAuthProxy checks that a per-server proxyOpt.AuthProxy
// enables proxy mode even when the process-global proxy.Opt.AuthProxy is empty,
// which is the normal case when the server is configured via serve/start.
func TestNewServerPerServerAuthProxy(t *testing.T) {
	fstest.Initialise()

	// Ensure the global is empty so we only test the per-server option.
	assert.Equal(t, "", proxy.Opt.AuthProxy)

	f, err := fs.NewFs(context.Background(), "testdata")
	require.NoError(t, err)

	opt := Opt
	opt.AuthKey = []string{"access-key,secret-key"}
	opt.HTTP.ListenAddr = []string{endpoint}

	proxyOpt := proxy.Opt
	proxyOpt.AuthProxy = "/path/to/auth/proxy"

	w, err := newServer(context.Background(), f, &opt, &vfscommon.Opt, &proxyOpt)
	require.NoError(t, err)
	defer func() {
		assert.NoError(t, w.Shutdown())
	}()
	assert.True(t, w.provider.IsProxy(), "expected auth proxy to be enabled by per-server option")
	assert.Nil(t, w.provider.VFS(), "expected no fixed VFS when auth proxy is in use")
}

// TestAuthProxyEmptySecret checks that a request for an arbitrary
// access key ID signed with an empty secret is refused when using an
// auth proxy without --auth-key.
func TestAuthProxyEmptySecret(t *testing.T) {
	fstest.Initialise()

	prog, err := filepath.Abs("../servetest/proxy_code.go")
	require.NoError(t, err)
	files, err := filepath.Abs("testdata")
	require.NoError(t, err)
	proxy.Opt.AuthProxy = "go run " + prog + " " + files
	defer func() {
		proxy.Opt.AuthProxy = ""
	}()

	endpoint, keyid, _, s := serveS3(t, nil)
	defer func() {
		assert.NoError(t, s.server.Shutdown())
	}()

	for _, accessKeyID := range []string{keyid, random.String(16)} {
		req, err := http.NewRequest("GET", endpoint+"/", nil)
		require.NoError(t, err)
		req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		signer := v4.NewSigner()
		err = signer.SignHTTP(context.Background(), aws.Credentials{AccessKeyID: accessKeyID, SecretAccessKey: ""}, req, "UNSIGNED-PAYLOAD", "s3", "us-east-1", time.Now())
		require.NoError(t, err)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		assert.Equal(t, http.StatusForbidden, resp.StatusCode, string(body))
		assert.NotContains(t, string(body), "ListAllMyBucketsResult")
	}
}

// TestAuthProxyKeysNotRegistered checks that secrets supplied by the
// auth proxy are never registered in gofakes3's process wide key
// store where any other serve s3 instance in the process would honour
// them.
func TestAuthProxyKeysNotRegistered(t *testing.T) {
	fstest.Initialise()

	prog, err := filepath.Abs("../servetest/proxy_code.go")
	require.NoError(t, err)
	files, err := filepath.Abs("testdata")
	require.NoError(t, err)
	proxy.Opt.AuthProxy = "go run " + prog + " " + files
	defer func() {
		proxy.Opt.AuthProxy = ""
	}()

	endpoint, keyid, keysec, s := serveS3(t, nil)
	defer func() {
		assert.NoError(t, s.server.Shutdown())
	}()

	sign := func() *http.Request {
		req, err := http.NewRequest("GET", endpoint+"/", nil)
		require.NoError(t, err)
		req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		signer := v4.NewSigner()
		err = signer.SignHTTP(context.Background(), aws.Credentials{AccessKeyID: keyid, SecretAccessKey: keysec}, req, "UNSIGNED-PAYLOAD", "s3", "us-east-1", time.Now())
		require.NoError(t, err)
		return req
	}

	// A correctly signed request is accepted by the server
	resp, err := http.DefaultClient.Do(sign())
	require.NoError(t, err)
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, string(body))

	// But the key it used must not be in the global key store
	assert.NotEqual(t, signature.ErrNone, signature.V4SignVerify(sign()), "proxy secret was registered in the gofakes3 key store")
}

// TestAuthKeyPerServer checks that two servers in the same process
// with different --auth-key pairs only accept their own credentials.
func TestAuthKeyPerServer(t *testing.T) {
	fstest.Initialise()
	f, err := fs.NewFs(context.Background(), "testdata")
	require.NoError(t, err)

	urlA, keyA, secA, sA := serveS3(t, f)
	defer func() { assert.NoError(t, sA.server.Shutdown()) }()
	urlB, keyB, secB, sB := serveS3(t, f)
	defer func() { assert.NoError(t, sB.server.Shutdown()) }()

	signedGet := func(url, accessKeyID, secret string) int {
		req, err := http.NewRequest("GET", url+"/", nil)
		require.NoError(t, err)
		req.Header.Set("X-Amz-Content-Sha256", "UNSIGNED-PAYLOAD")
		err = v4.NewSigner().SignHTTP(context.Background(), aws.Credentials{AccessKeyID: accessKeyID, SecretAccessKey: secret}, req, "UNSIGNED-PAYLOAD", "s3", "us-east-1", time.Now())
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		_, _ = io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		return resp.StatusCode
	}

	assert.Equal(t, http.StatusOK, signedGet(urlA, keyA, secA), "A with A's key")
	assert.Equal(t, http.StatusOK, signedGet(urlB, keyB, secB), "B with B's key")
	assert.Equal(t, http.StatusForbidden, signedGet(urlA, keyB, secB), "A with B's key")
	assert.Equal(t, http.StatusForbidden, signedGet(urlB, keyA, secA), "B with A's key")
}

func TestRc(t *testing.T) {
	servetest.TestRc(t, rc.Params{
		"type":           "s3",
		"vfs_cache_mode": "off",
	})
}
