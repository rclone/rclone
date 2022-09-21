// Serve s3 tests set up a server and run the integration tests
// for the s3 remote against it.

package s3

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	_ "github.com/rclone/rclone/backend/local"
	"github.com/rclone/rclone/cmd/serve/servetest"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/fstest"
	httplib "github.com/rclone/rclone/lib/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	endpoint = "localhost:0"
)

// TestS3 runs the s3 server then runs the unit tests for the
// s3 remote against it.
func TestS3(t *testing.T) {
	// Configure and start the server
	start := func(f fs.Fs) (configmap.Simple, func()) {
		keyid := RandString(16)
		keysec := RandString(16)
		serveropt := &Options{
			HTTP:           httplib.DefaultCfg(),
			pathBucketMode: true,
			hashName:       "",
			hashType:       hash.None,
			authPair:       []string{fmt.Sprintf("%s,%s", keyid, keysec)},
		}

		serveropt.HTTP.ListenAddr = []string{endpoint}
		w, err := newServer(context.Background(), f, serveropt)
		router := w.Router()
		assert.NoError(t, err)

		w.Bind(router)
		w.Serve()
		testURL := w.Server.URLs()[0]
		// Config for the backend we'll use to connect to the server
		config := configmap.Simple{
			"type":              "s3",
			"provider":          "Rclone",
			"endpoint":          testURL,
			"list_url_encode":   "true",
			"access_key_id":     keyid,
			"secret_access_key": keysec,
		}

		return config, func() {}
	}

	Run(t, "s3", start)
}

func RandString(n int) string {
	src := rand.New(rand.NewSource(time.Now().UnixNano()))
	b := make([]byte, (n+1)/2)

	if _, err := src.Read(b); err != nil {
		panic(err)
	}

	return hex.EncodeToString(b)[:n]
}

func Run(t *testing.T, name string, start servetest.StartFn) {
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

	// Run the backend tests with an on the fly remote
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

	// Run the test
	out, err := cmd.CombinedOutput()
	if len(out) != 0 {
		t.Logf("\n----------\n%s----------\n", string(out))
	}
	assert.NoError(t, err, "Running "+name+" integration tests")

}
