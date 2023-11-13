// Serve s3 tests set up a server and run the integration tests
// for the s3 remote against it.

package s3

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path"
	"strings"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/rclone/rclone/fs/object"

	_ "github.com/rclone/rclone/backend/local"
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
func serveS3(f fs.Fs) (testURL string, keyid string, keysec string) {
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
	w, _ := newServer(context.Background(), f, serveropt)
	router := w.Router()

	w.Bind(router)
	w.Serve()
	testURL = w.Server.URLs()[0]

	return
}

// TestS3 runs the s3 server then runs the unit tests for the
// s3 remote against it.
func TestS3(t *testing.T) {
	start := func(f fs.Fs) (configmap.Simple, func()) {
		testURL, keyid, keysec := serveS3(f)
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

	RunS3UnitTests(t, "s3", start)
}

func RunS3UnitTests(t *testing.T, name string, start servetest.StartFn) {
	fstest.Initialise()
	ci := fs.GetConfig(context.Background())
	ci.DisableFeatures = append(ci.DisableFeatures, "Metadata")

	fremote, _, clean, err := fstest.RandomRemote()
	assert.NoError(t, err)
	defer clean()

	err = fremote.Mkdir(context.Background(), "")
	assert.NoError(t, err)

	f := fremote
	config, cleanup := start(f)
	defer cleanup()

	// Change directory to run the tests
	cwd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir("../../../backend/" + name)
	require.NoError(t, err, "failed to cd to "+name+" backend")
	defer func() {
		// Change back to the old directory
		require.NoError(t, os.Chdir(cwd))
	}()

	// RunS3UnitTests the backend tests with an on the fly remote
	args := []string{"test"}
	if testing.Verbose() {
		args = append(args, "-v")
	}
	if *fstest.Verbose {
		args = append(args, "-verbose")
	}
	remoteName := name + "test:"
	args = append(args, "-remote", remoteName)
	args = append(args, "-run", "^TestIntegration$")
	args = append(args, "-list-retries", fmt.Sprint(*fstest.ListRetries))
	cmd := exec.Command("go", args...)

	// Configure the backend with environment variables
	cmd.Env = os.Environ()
	prefix := "RCLONE_CONFIG_" + strings.ToUpper(remoteName[:len(remoteName)-1]) + "_"
	for k, v := range config {
		cmd.Env = append(cmd.Env, prefix+strings.ToUpper(k)+"="+v)
	}

	// RunS3UnitTests the test
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running "+name+" integration tests")
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

			endpoint, keyid, keysec := serveS3(f)
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
