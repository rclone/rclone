// Test Cryptomator filesystem interface
package cryptomator_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/backend/cryptomator"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fs/rc/webgui"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/fstest/fstests"
	"github.com/rclone/rclone/lib/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/alias"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/webdav"
)

var (
	UnimplementableFsMethods = []string{
		// TODO: implement these:
		// It's not possible to complete this in one call, but Purge could still be implemented more efficiently than the fallback by
		// recursing and deleting a full directory at a time (instead of each file individually.)
		"Purge",
		// MergeDirs could be implemented by merging the underlying directories, while taking care to leave the dirid.c9r alone.
		"MergeDirs",
		// OpenWriterAt could be implemented by a strategy such as: to write to a chunk, read and decrypt it, handle all writes, then reencrypt and upload.
		"OpenWriterAt",
		// OpenChunkWriter could be implemented, at least if the backend's chunk size is a multiple of Cryptomator's chunk size.
		"OpenChunkWriter",

		// Having ListR on the backend doesn't help at all for implementing it in Cryptomator.
		"ListR",
		// ChangeNotify would have to undo the dir to dir ID conversion, which is lossy. It can be done, but not without scanning and caching the full hierarchy.
		"ChangeNotify",
	}
)

// TestIntegration runs integration tests against the remote
func TestIntegration(t *testing.T) {
	if *fstest.RemoteName == "" {
		t.Skip("Skipping as -remote not set")
	}
	fstests.Run(t, &fstests.Opt{
		RemoteName:               *fstest.RemoteName,
		NilObject:                (*cryptomator.DecryptingObject)(nil),
		TiersToTest:              []string{"REDUCED_REDUNDANCY", "STANDARD"},
		UnimplementableFsMethods: UnimplementableFsMethods,
	})
}

// TestStandard runs integration tests against the remote
func TestStandard(t *testing.T) {
	if *fstest.RemoteName != "" {
		t.Skip("Skipping as -remote set")
	}
	tempdir := filepath.Join(os.TempDir(), "rclone-cryptomator-test-standard")
	name := "TestCryptomator"
	fstests.Run(t, &fstests.Opt{
		RemoteName: name + ":",
		NilObject:  (*cryptomator.DecryptingObject)(nil),
		ExtraConfig: []fstests.ExtraConfigItem{
			{Name: name, Key: "type", Value: "cryptomator"},
			{Name: name, Key: "remote", Value: tempdir},
			{Name: name, Key: "password", Value: obscure.MustObscure("potato")},
		},
		QuickTestOK:              true,
		UnimplementableFsMethods: UnimplementableFsMethods,
	})
}

func runCryptomator(ctx context.Context, t *testing.T, vaultPath string, password string) string {
	// Download
	cryptomatorCliDownload := map[string]map[string]string{
		"darwin": {
			"arm64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-mac-arm64.zip",
			"amd64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-mac-x64.zip",
		},
		"linux": {
			"arm64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-linux-arm64.zip",
			"amd64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-linux-x64.zip",
		},
	}
	var dlURL string
	if archMap, ok := cryptomatorCliDownload[runtime.GOOS]; ok {
		if url, ok := archMap[runtime.GOARCH]; ok {
			dlURL = url
		}
	}
	if dlURL == "" {
		t.Skipf("no cryptomator download available for GOOS=%s GOOARCH=%s, skipping", runtime.GOOS, runtime.GOARCH)
	}
	cacheDir := filepath.Join(config.GetCacheDir(), "test-cryptomator")
	zipPath := filepath.Join(cacheDir, path.Base(dlURL))
	extractDir := filepath.Join(cacheDir, "bin")
	err := file.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)

	if _, err := os.Stat(zipPath); err != nil {
		t.Logf("will download cryptomator from %s to %s", dlURL, zipPath)
		err = os.RemoveAll(zipPath)
		require.NoError(t, err)
		err = webgui.DownloadFile(zipPath, dlURL)
		require.NoError(t, err)
		err = os.RemoveAll(extractDir)
		require.NoError(t, err)
	}
	if _, err := os.Stat(extractDir); err != nil {
		t.Logf("will extract from %s to %s", zipPath, extractDir)
		err = webgui.Unzip(zipPath, extractDir)
		require.NoError(t, err)
	}
	t.Logf("have cryptomator cli at %q", extractDir)

	// Run
	var exe string
	switch runtime.GOOS {
	case "darwin":
		exe = filepath.Join(extractDir, "cryptomator-cli.app", "Contents", "MacOS", "cryptomator-cli")
	case "linux":
		exe = filepath.Join(extractDir, "cryptomator-cli", "bin", "cryptomator-cli")
	}
	err = os.Chmod(exe, 0755)
	require.NoError(t, err)
	cmd := exec.CommandContext(
		ctx,
		exe,
		"unlock", vaultPath,
		"--mounter=org.cryptomator.frontend.webdav.mount.FallbackMounter",
		"--password:env=CRYPTOMATOR_PASSWORD",
	)
	cmd.Env = append(cmd.Env, fmt.Sprintf("CRYPTOMATOR_PASSWORD=%s", password))
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	err = cmd.Start()
	require.NoError(t, err)
	re := regexp.MustCompile(`Unlocked and mounted vault successfully to (\S+)`)
	webdavURL := make(chan string)
	go func() {
		scanner := bufio.NewScanner(stdout)
		done := false
		for scanner.Scan() {
			t.Log(scanner.Text())
			if done {
				continue
			}
			matches := re.FindSubmatch([]byte(scanner.Text()))
			if matches != nil {
				webdavURL <- string(matches[1])
				done = true
			}
		}
	}()

	return <-webdavURL
}

// TestAgainstCryptomator tests rclone against the Cryptomator CLI
func TestAgainstCryptomator(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	localPath, err := fstest.LocalRemote()
	require.NoError(t, err)
	password := "potato"
	t.Cleanup(func() {
		_ = os.RemoveAll(localPath)
	})

	fstest.Initialise()
	config.FileSetValue("TestCryptomatorRclone", "type", "cryptomator")
	config.FileSetValue("TestCryptomatorRclone", "remote", localPath)
	config.FileSetValue("TestCryptomatorRclone", "password", obscure.MustObscure(password))
	rcloneFs, err := fs.NewFs(ctx, "TestCryptomatorRclone:")
	require.NoError(t, err)

	webdavURL := runCryptomator(ctx, t, localPath, password)
	config.FileSetValue("TestCryptomatorCli", "type", "webdav")
	config.FileSetValue("TestCryptomatorCli", "url", webdavURL)
	cryptomFs, err := fs.NewFs(ctx, "TestCryptomatorCli:")
	require.NoError(t, err)

	check := func(items []fstest.Item, dirs []string) {
		t.Logf("comparing %v with %v", cryptomFs, rcloneFs)
		fstest.CheckListingWithPrecision(t, rcloneFs, items, dirs, fs.GetModifyWindow(ctx, cryptomFs))
		fstest.CheckListingWithPrecision(t, cryptomFs, items, dirs, fs.GetModifyWindow(ctx, cryptomFs))

		buf := &bytes.Buffer{}
		err = operations.CheckDownload(ctx, &operations.CheckOpt{
			Fdst:     cryptomFs,
			Fsrc:     rcloneFs,
			Combined: buf,
		})
		scan := bufio.NewScanner(buf)
		for scan.Scan() {
			line := scan.Text()
			if strings.HasPrefix(line, "= ") {
				t.Log(line)
				continue
			}
			t.Error(line)
		}
		assert.NoError(t, err)
		t.Logf("matched %v %v", items, dirs)
	}

	put := func(fs fs.Fs, path string, content string) (fstest.Item, fs.Object) {
		now := time.Now()
		obj, err := fs.Put(ctx, bytes.NewBufferString(content), object.NewStaticObjectInfo(path, time.Now(), -1, true, nil, nil))
		assert.NoError(t, err)
		item := fstest.NewItem(path, content, now)
		return item, obj
	}
	get := func(fs fs.Fs, path string) fs.Object {
		obj, err := fs.NewObject(ctx, path)
		assert.NoError(t, err)
		return obj
	}
	rclone1, _ := put(rcloneFs, "rclone1", "testing 123")
	cryptom1, _ := put(cryptomFs, "cryptom1", "testing 456")
	check([]fstest.Item{rclone1, cryptom1}, []string{})

	err = rcloneFs.Mkdir(ctx, "rclone2")
	assert.NoError(t, err)
	err = cryptomFs.Mkdir(ctx, "cryptom2")
	assert.NoError(t, err)
	check([]fstest.Item{rclone1, cryptom1}, []string{"rclone2", "cryptom2"})

	_, err = rcloneFs.Features().Move(ctx, get(rcloneFs, "cryptom1"), "rclone2/cryptom1")
	assert.NoError(t, err)
	rclone1.Path = "cryptom2/rclone1"
	_, err = cryptomFs.Features().Move(ctx, get(cryptomFs, "rclone1"), "cryptom2/rclone1")
	assert.NoError(t, err)
	cryptom1.Path = "rclone2/cryptom1"
	check([]fstest.Item{rclone1, cryptom1}, []string{"rclone2", "cryptom2"})

	err = get(cryptomFs, rclone1.Path).Remove(ctx)
	assert.NoError(t, err)
	check([]fstest.Item{cryptom1}, []string{"rclone2", "cryptom2"})
	err = get(rcloneFs, cryptom1.Path).Remove(ctx)
	assert.NoError(t, err)
	check([]fstest.Item{}, []string{"rclone2", "cryptom2"})
}
