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
	config.FileSetValue("TestCryptomatorRclone", "auto_create", "true")

	// Create vault before starting cryptomator - for some reason cryptomator cli cannot create vaults
	_, err = fs.NewFs(t.Context(), "TestCryptomatorRclone:")
	require.NoError(t, err)

	webdavURL := runCryptomator(t, localPath, password)
	config.FileSetValue("TestCryptomatorCli", "type", "webdav")
	config.FileSetValue("TestCryptomatorCli", "url", webdavURL)

	check := func(t *testing.T, desc string, rcloneFs, cryptomFs fs.Fs, items []fstest.Item, dirs []string) {
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

	makeFses := func(t *testing.T) (rcloneFs, cryptomFs fs.Fs, checkFn func(t *testing.T, desc string, items []fstest.Item, dirs []string)) {
		rcloneFs, err = fs.NewFs(t.Context(), "TestCryptomatorRclone:"+t.Name())
		require.NoError(t, err)
		// Must create the directory before opening it in webdav because of a bug in cryptomator cli (seen on v0.6.1):
		// if you start the webdav server, list a nonexistent directory, then create the directory externally to the webdav server, then try to list it again, the server won't show the directory and complains that the pointer points to a nonexistent directory id; but the hashed encrypted directory id in the log is entirely different from what it should be. Reads fine if you restart the server.
		err = rcloneFs.Mkdir(t.Context(), "")
		require.NoError(t, err)
		cryptomFs, err = fs.NewFs(t.Context(), "TestCryptomatorCli:"+t.Name())
		require.NoError(t, err)
		checkFn = func(t *testing.T, desc string, items []fstest.Item, dirs []string) {
			check(t, desc, rcloneFs, cryptomFs, items, dirs)
		}
		return
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
	remove := func(t *testing.T, fs fs.Fs, path string) {
		err := get(t, fs, path).Remove(t.Context())
		assert.NoError(t, err)
	}
	move := func(t *testing.T, fs fs.Fs, item *fstest.Item, dest string) {
		_, err := fs.Features().Move(t.Context(), get(t, fs, item.Path), dest)
		assert.NoError(t, err)
		item.Path = dest
	}
	copy := func(t *testing.T, fs fs.Fs, item fstest.Item, dest string) fstest.Item {
		_, err := fs.Features().Copy(t.Context(), get(t, fs, item.Path), dest)
		assert.NoError(t, err)
		item.Path = dest
		return item
	}

	testCases := func(t *testing.T, filenameSuffix string) {
		ctx := t.Context()
		rcloneFs, cryptomFs, check := makeFses(t)

		n := map[string]string{}
		names := []string{"rclone1", "rclone2", "cryptom1", "cryptom2"}
		for _, name := range names {
			n[name] = name + filenameSuffix
		}
		for _, name1 := range names {
			for _, name2 := range names {
				n[name1+"/"+name2] = n[name1] + "/" + n[name2]
			}
		}

		rclone1, _ := put(t, rcloneFs, n["rclone1"], "testing 123")
		check(t, "put with rclone", []fstest.Item{rclone1}, []string{})
		cryptom1, _ := put(t, cryptomFs, n["cryptom1"], "testing 456")
		check(t, "put with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{})

		err = rcloneFs.Mkdir(ctx, n["rclone2"])
		assert.NoError(t, err)
		check(t, "mkdir with rclone", []fstest.Item{rclone1, cryptom1}, []string{n["rclone2"]})
		err = cryptomFs.Mkdir(ctx, n["cryptom2"])
		assert.NoError(t, err)
		check(t, "mkdir with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{n["rclone2"], n["cryptom2"]})

		move(t, rcloneFs, &cryptom1, n["rclone2/cryptom1"])
		check(t, "move with rclone", []fstest.Item{rclone1, cryptom1}, []string{n["rclone2"], n["cryptom2"]})
		move(t, rcloneFs, &rclone1, n["cryptom2/rclone1"])
		check(t, "move with cryptomator", []fstest.Item{rclone1, cryptom1}, []string{n["rclone2"], n["cryptom2"]})

		remove(t, cryptomFs, rclone1.Path)
		check(t, "remove with cryptomator", []fstest.Item{cryptom1}, []string{n["rclone2"], n["cryptom2"]})
		remove(t, rcloneFs, cryptom1.Path)
		check(t, "remove with rclone", []fstest.Item{}, []string{n["rclone2"], n["cryptom2"]})

		err = rcloneFs.Rmdir(ctx, n["cryptom2"])
		assert.NoError(t, err)
		check(t, "rmdir with rclone", []fstest.Item{}, []string{n["rclone2"]})
		err = cryptomFs.Rmdir(ctx, n["rclone2"])
		assert.NoError(t, err)
		check(t, "rmdir with cryptomator", []fstest.Item{}, []string{})
	}

	t.Run("Basic", func(t *testing.T) {
		testCases(t, ".")
	})
	t.Run("LongNames", func(t *testing.T) {
		testCases(t, strings.Repeat(".", 1000))
	})

	t.Run("CopyMove", func(t *testing.T) {
		// ctx := t.Context()
		rcloneFs, _, check := makeFses(t)
		long := func(name string) string {
			return name + strings.Repeat(".", 1000)
		}

		file, _ := put(t, rcloneFs, "file", "testing 123")
		check(t, "put file", []fstest.Item{file}, []string{})

		t.Run("Move", func(t *testing.T) {
			move(t, rcloneFs, &file, "file2")
			check(t, "move file", []fstest.Item{file}, []string{})
			move(t, rcloneFs, &file, long("file"))
			check(t, "move file to long name", []fstest.Item{file}, []string{})
			move(t, rcloneFs, &file, long("file2"))
			check(t, "move file to other long name", []fstest.Item{file}, []string{})
			move(t, rcloneFs, &file, "file")
			check(t, "move file to short name", []fstest.Item{file}, []string{})
		})

		t.Run("Copy", func(t *testing.T) {
			file2 := copy(t, rcloneFs, file, "file2")
			check(t, "copy file", []fstest.Item{file, file2}, []string{})
			longfile := copy(t, rcloneFs, file2, long("file"))
			check(t, "copy file to long name", []fstest.Item{file, file2, longfile}, []string{})
			longfile2 := copy(t, rcloneFs, longfile, long("file2"))
			check(t, "copy file to other long name", []fstest.Item{file, file2, longfile, longfile2}, []string{})
			remove(t, rcloneFs, file.Path)
			file = copy(t, rcloneFs, longfile2, "file")
			check(t, "copy file to short name", []fstest.Item{file, file2, longfile, longfile2}, []string{})

			remove(t, rcloneFs, file2.Path)
			remove(t, rcloneFs, longfile.Path)
			remove(t, rcloneFs, longfile2.Path)
		})

		t.Run("DirMove", func(t *testing.T) {
			ctx := t.Context()
			err := rcloneFs.Mkdir(ctx, "dir")
			assert.NoError(t, err)
			check(t, "mkdir", []fstest.Item{file}, []string{"dir"})

			err = rcloneFs.Features().DirMove(ctx, rcloneFs, "dir", "dir2")
			assert.NoError(t, err)
			check(t, "dirmove", []fstest.Item{file}, []string{"dir2"})

			err = rcloneFs.Features().DirMove(ctx, rcloneFs, "dir2", long("dir"))
			assert.NoError(t, err)
			check(t, "dirmove to long name", []fstest.Item{file}, []string{long("dir")})

			err = rcloneFs.Features().DirMove(ctx, rcloneFs, long("dir"), long("dir2"))
			assert.NoError(t, err)
			check(t, "dirmove to other long name", []fstest.Item{file}, []string{long("dir2")})

			err = rcloneFs.Features().DirMove(ctx, rcloneFs, long("dir2"), "dir")
			assert.NoError(t, err)
			check(t, "dirmove to short name", []fstest.Item{file}, []string{"dir"})

			err = rcloneFs.Rmdir(ctx, "dir")
			assert.NoError(t, err)
		})
	})
}
