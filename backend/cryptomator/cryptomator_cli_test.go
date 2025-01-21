package cryptomator_test

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rclone/rclone/cmd/archive/extract"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/obscure"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
	"github.com/rclone/rclone/fstest"
	"github.com/rclone/rclone/lib/file"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "github.com/rclone/rclone/backend/webdav"
)

func runCryptomator(t *testing.T, vaultPath string, password string) string {
	ctx := t.Context()

	// Download
	cryptomatorCliDownload := map[string]map[string]string{
		"darwin": {
			"arm64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-mac-arm64.zip",
			"amd64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-mac-x64.zip",
		},
		"linux": {
			"arm64": "https://github.com/cryptomator/cli/releases/download/0.6.1/cryptomator-cli-0.6.1-linux-aarch64.zip",
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
	zipPath := filepath.Join(cacheDir, filepath.Base(dlURL))
	extractDir := filepath.Join(cacheDir, "bin")
	err := file.MkdirAll(cacheDir, 0755)
	require.NoError(t, err)
	var exe string
	switch runtime.GOOS {
	case "darwin":
		exe = filepath.Join(extractDir, "cryptomator-cli.app", "Contents", "MacOS", "cryptomator-cli")
	case "linux":
		exe = filepath.Join(extractDir, "cryptomator-cli", "bin", "cryptomator-cli")
	}

	if _, err := os.Stat(exe); err != nil {
		t.Logf("will download cryptomator from %s", dlURL)
		out, err := os.Create(zipPath)
		require.NoError(t, err)
		err = operations.CopyURLToWriter(ctx, dlURL, out)
		require.NoError(t, err)
		err = out.Close()
		require.NoError(t, err)

		t.Logf("will extract from %s to %s", zipPath, extractDir)
		require.Equal(t, filepath.Dir(extractDir), filepath.Dir(zipPath))
		cacheFs, err := fs.NewFs(ctx, filepath.Dir(extractDir))
		require.NoError(t, err)
		err = extract.ArchiveExtract(ctx, cacheFs, filepath.Base(extractDir), cacheFs, filepath.Base(zipPath))
		require.NoError(t, err)

		err = os.Chmod(exe, 0755)
		require.NoError(t, err)
	}
	t.Logf("have cryptomator cli at %q", extractDir)

	// Run
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

	// Create vault before starting cryptomator - for some reason cryptomator cli cannot create vaults
	_, err = fs.NewFs(t.Context(), "TestCryptomatorRclone:")
	require.NoError(t, err)

	webdavURL := runCryptomator(t, localPath, password)
	config.FileSetValue("TestCryptomatorCli", "type", "webdav")
	config.FileSetValue("TestCryptomatorCli", "url", webdavURL)

	makeFses := func(t *testing.T) (rcloneFs, cryptomFs fs.Fs) {
		rcloneFs, err = fs.NewFs(t.Context(), "TestCryptomatorRclone:"+t.Name())
		require.NoError(t, err)
		// Must create the directory before opening it in webdav because of a bug in cryptomator cli (seen on v0.6.1):
		// if you start the webdav server, list a nonexistent directory, then create the directory externally to the webdav server, then try to list it again, the server won't show the directory and complains that the pointer points to a nonexistent directory id; but the hashed encrypted directory id in the log is entirely different from what it should be. Reads fine if you restart the server.
		err = rcloneFs.Mkdir(t.Context(), "")
		require.NoError(t, err)
		cryptomFs, err = fs.NewFs(t.Context(), "TestCryptomatorCli:"+t.Name())
		require.NoError(t, err)
		return
	}
	rcloneFs, cryptomFs := makeFses(t)

	check := func(desc string, items []fstest.Item, dirs []string) {
		t.Logf("after %s: comparing %v with %v", desc, cryptomFs, rcloneFs)
		precision := fs.GetModifyWindow(t.Context(), cryptomFs, cryptomFs)
		fstest.CheckListingWithPrecision(t, cryptomFs, items, dirs, precision)
		fstest.CheckListingWithPrecision(t, rcloneFs, items, dirs, precision)

		buf := &bytes.Buffer{}
		err = operations.CheckDownload(t.Context(), &operations.CheckOpt{
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

	put := func(t *testing.T, fs fs.Fs, path string, content string) (fstest.Item, fs.Object) {
		now := time.Now()
		obj, err := fs.Put(t.Context(), bytes.NewBufferString(content), object.NewStaticObjectInfo(path, time.Now(), -1, true, nil, nil))
		assert.NoError(t, err)
		item := fstest.NewItem(path, content, now)
		return item, obj
	}
	get := func(t *testing.T, fs fs.Fs, path string) fs.Object {
		obj, err := fs.NewObject(t.Context(), path)
		assert.NoError(t, err)
		return obj
	}

	ctx := t.Context()

	rclone1, _ := put(t, rcloneFs, "rclone1", "testing 123")
	check("put with rclone", []fstest.Item{rclone1}, []string{})
	cryptom1, _ := put(t, cryptomFs, "cryptom1", "testing 456")
	check("put with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{})

	err = rcloneFs.Mkdir(ctx, "rclone2")
	assert.NoError(t, err)
	check("mkdir with rclone", []fstest.Item{rclone1, cryptom1}, []string{"rclone2"})
	err = cryptomFs.Mkdir(ctx, "cryptom2")
	assert.NoError(t, err)
	check("mkdir with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{"rclone2", "cryptom2"})

	_, err = rcloneFs.Features().Move(ctx, get(t, rcloneFs, "cryptom1"), "rclone2/cryptom1")
	assert.NoError(t, err)
	cryptom1.Path = "rclone2/cryptom1"
	check("move with rclone", []fstest.Item{rclone1, cryptom1}, []string{"rclone2", "cryptom2"})
	_, err = cryptomFs.Features().Move(ctx, get(t, cryptomFs, "rclone1"), "cryptom2/rclone1")
	assert.NoError(t, err)
	rclone1.Path = "cryptom2/rclone1"
	check("move with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{"rclone2", "cryptom2"})

	err = get(t, cryptomFs, rclone1.Path).Remove(ctx)
	assert.NoError(t, err)
	check("remove with cryptomator", []fstest.Item{cryptom1}, []string{"rclone2", "cryptom2"})
	err = get(t, rcloneFs, cryptom1.Path).Remove(ctx)
	assert.NoError(t, err)
	check("remove with rclone", []fstest.Item{}, []string{"rclone2", "cryptom2"})

	err = rcloneFs.Rmdir(ctx, "cryptom2")
	assert.NoError(t, err)
	check("rmdir with rclone", []fstest.Item{}, []string{"rclone2"})
	err = cryptomFs.Rmdir(ctx, "rclone2")
	assert.NoError(t, err)
	check("rmdir with cryptomator", []fstest.Item{}, []string{})
}
