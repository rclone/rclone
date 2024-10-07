package configfile

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var configData = `[one]
type = number1
fruit = potato

[two]
type = number2
fruit = apple
topping = nuts

[three]
type = number3
fruit = banana

`

// Fill up a temporary config file with the testdata filename passed in
func setConfigFile(t *testing.T, data string) func() {
	out, err := os.CreateTemp("", "rclone-configfile-test")
	require.NoError(t, err)
	filePath := out.Name()

	_, err = out.Write([]byte(data))
	require.NoError(t, err)

	require.NoError(t, out.Close())

	old := config.GetConfigPath()
	assert.NoError(t, config.SetConfigPath(filePath))
	return func() {
		assert.NoError(t, config.SetConfigPath(old))
		_ = os.Remove(filePath)
	}
}

// toUnix converts \r\n to \n in buf
func toUnix(buf string) string {
	if runtime.GOOS == "windows" {
		return strings.ReplaceAll(buf, "\r\n", "\n")
	}
	return buf
}

func TestConfigFile(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, data.Load())

	t.Run("Read", func(t *testing.T) {
		t.Run("Serialize", func(t *testing.T) {
			buf, err := data.Serialize()
			require.NoError(t, err)
			assert.Equal(t, configData, toUnix(buf))
		})
		t.Run("HasSection", func(t *testing.T) {
			assert.True(t, data.HasSection("one"))
			assert.False(t, data.HasSection("missing"))
		})
		t.Run("GetSectionList", func(t *testing.T) {
			assert.Equal(t, []string{
				"one",
				"two",
				"three",
			}, data.GetSectionList())
		})
		t.Run("GetKeyList", func(t *testing.T) {
			assert.Equal(t, []string{
				"type",
				"fruit",
				"topping",
			}, data.GetKeyList("two"))
			assert.Equal(t, []string(nil), data.GetKeyList("unicorn"))
		})
		t.Run("GetValue", func(t *testing.T) {
			value, ok := data.GetValue("one", "type")
			assert.True(t, ok)
			assert.Equal(t, "number1", value)
			value, ok = data.GetValue("three", "fruit")
			assert.True(t, ok)
			assert.Equal(t, "banana", value)
			value, ok = data.GetValue("one", "typeX")
			assert.False(t, ok)
			assert.Equal(t, "", value)
			value, ok = data.GetValue("threeX", "fruit")
			assert.False(t, ok)
			assert.Equal(t, "", value)
		})
	})

	//defer setConfigFile(configData)()

	t.Run("Write", func(t *testing.T) {
		t.Run("SetValue", func(t *testing.T) {
			data.SetValue("one", "extra", "42")
			data.SetValue("two", "fruit", "acorn")

			buf, err := data.Serialize()
			require.NoError(t, err)
			assert.Equal(t, `[one]
type = number1
fruit = potato
extra = 42

[two]
type = number2
fruit = acorn
topping = nuts

[three]
type = number3
fruit = banana

`, toUnix(buf))
			t.Run("DeleteKey", func(t *testing.T) {
				data.DeleteKey("one", "type")
				data.DeleteKey("two", "missing")
				data.DeleteKey("three", "fruit")
				buf, err := data.Serialize()
				require.NoError(t, err)
				assert.Equal(t, `[one]
fruit = potato
extra = 42

[two]
type = number2
fruit = acorn
topping = nuts

[three]
type = number3

`, toUnix(buf))
				t.Run("DeleteSection", func(t *testing.T) {
					data.DeleteSection("two")
					data.DeleteSection("missing")
					buf, err := data.Serialize()
					require.NoError(t, err)
					assert.Equal(t, `[one]
fruit = potato
extra = 42

[three]
type = number3

`, toUnix(buf))
					t.Run("Save", func(t *testing.T) {
						require.NoError(t, data.Save())
						buf, err := os.ReadFile(config.GetConfigPath())
						require.NoError(t, err)
						assert.Equal(t, `[one]
fruit = potato
extra = 42

[three]
type = number3

`, toUnix(string(buf)))
					})
				})
			})
		})
	})
}

func TestConfigFileReload(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, data.Load())

	value, ok := data.GetValue("three", "appended")
	assert.False(t, ok)
	assert.Equal(t, "", value)

	// Now write a new value on the end
	out, err := os.OpenFile(config.GetConfigPath(), os.O_APPEND|os.O_WRONLY, 0777)
	require.NoError(t, err)
	_, err = fmt.Fprintln(out, "appended = what magic")
	require.NoError(t, err)
	require.NoError(t, out.Close())

	// And check we magically reloaded it
	value, ok = data.GetValue("three", "appended")
	assert.True(t, ok)
	assert.Equal(t, "what magic", value)
}

func TestConfigFileDoesNotExist(t *testing.T) {
	defer setConfigFile(t, configData)()
	data := &Storage{}

	require.NoError(t, os.Remove(config.GetConfigPath()))

	err := data.Load()
	require.Equal(t, config.ErrorConfigFileNotFound, err)

	// check that using data doesn't crash
	value, ok := data.GetValue("three", "appended")
	assert.False(t, ok)
	assert.Equal(t, "", value)
}

func testConfigFileNoConfig(t *testing.T, configPath string) {
	assert.NoError(t, config.SetConfigPath(configPath))
	data := &Storage{}

	err := data.Load()
	require.Equal(t, config.ErrorConfigFileNotFound, err)

	data.SetValue("one", "extra", "42")
	value, ok := data.GetValue("one", "extra")
	assert.True(t, ok)
	assert.Equal(t, "42", value)

	err = data.Save()
	require.Error(t, err)
}

func TestConfigFileNoConfig(t *testing.T) {
	old := config.GetConfigPath()
	defer func() {
		assert.NoError(t, config.SetConfigPath(old))
	}()

	t.Run("Empty", func(t *testing.T) {
		testConfigFileNoConfig(t, "")
	})
	t.Run("NotFound", func(t *testing.T) {
		testConfigFileNoConfig(t, "/notfound")
	})
}

func TestConfigFileSave(t *testing.T) {
	testDir := t.TempDir()
	configPath := filepath.Join(testDir, "a", "b", "c", "configfile")

	assert.NoError(t, config.SetConfigPath(configPath))
	data := &Storage{}
	require.Error(t, data.Load(), config.ErrorConfigFileNotFound)

	t.Run("CreatesDirsAndFile", func(t *testing.T) {
		err := data.Save()
		require.NoError(t, err)
		info, err := os.Stat(configPath)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
	})
	t.Run("KeepsFileMode", func(t *testing.T) {
		if runtime.GOOS != "linux" {
			t.Skip("this is a Linux only test")
		}
		assert.NoError(t, os.Chmod(configPath, 0400)) // -r--------
		defer func() {
			_ = os.Chmod(configPath, 0644) // -rw-r--r--
		}()
		err := data.Save()
		require.NoError(t, err)
		info, err := os.Stat(configPath)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0400), info.Mode().Perm())
	})
	t.Run("SucceedsEvenIfReadOnlyFile", func(t *testing.T) {
		// Save succeeds even if file is read-only since it does not write directly to the file.
		assert.NoError(t, os.Chmod(configPath, 0400)) // -r--------
		defer func() {
			_ = os.Chmod(configPath, 0644) // -rw-r--r--
		}()
		err := data.Save()
		assert.NoError(t, err)
	})
	t.Run("FailsIfNotAccessToDir", func(t *testing.T) {
		// Save fails if no access to the directory.
		if runtime.GOOS != "linux" {
			// On Windows the os.Chmod only affects the read-only attribute of files)
			t.Skip("this is a Linux only test")
		}
		configDir := filepath.Dir(configPath)
		assert.NoError(t, os.Chmod(configDir, 0400)) // -r--------
		defer func() {
			_ = os.Chmod(configDir, 0755) // -rwxr-xr-x
		}()
		err := data.Save()
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "failed to resolve config file path"))
	})
	t.Run("FailsIfNotAllowedToCreateNewFiles", func(t *testing.T) {
		// Save fails if read-only access to the directory, since it needs to create temporary files in there.
		if runtime.GOOS != "linux" {
			// On Windows the os.Chmod only affects the read-only attribute of files)
			t.Skip("this is a Linux only test")
		}
		configDir := filepath.Dir(configPath)
		assert.NoError(t, os.Chmod(configDir, 0544)) // -r-xr--r--
		defer func() {
			_ = os.Chmod(configDir, 0755) // -rwxr-xr-x
		}()
		err := data.Save()
		require.Error(t, err)
		assert.True(t, strings.HasPrefix(err.Error(), "failed to create temp file for new config"))
	})
}

func TestConfigFileSaveSymlinkAbsolute(t *testing.T) {
	if runtime.GOOS != "linux" {
		// Symlinks may require admin privileges on Windows and os.Symlink will then
		// fail with "A required privilege is not held by the client."
		t.Skip("this is a Linux only test")
	}
	testDir := t.TempDir()
	linkDir := filepath.Join(testDir, "a")
	err := os.Mkdir(linkDir, os.ModePerm)
	require.NoError(t, err)

	testSymlink := func(t *testing.T, link string, target string, resolvedTarget string) {
		err = os.Symlink(target, link)
		require.NoError(t, err)
		defer func() {
			_ = os.Remove(link)
		}()

		assert.NoError(t, config.SetConfigPath(link))
		data := &Storage{}
		require.Error(t, data.Load(), config.ErrorConfigFileNotFound)

		err = data.Save()
		require.NoError(t, err)

		info, err := os.Lstat(link)
		require.NoError(t, err)
		assert.True(t, info.Mode()&os.ModeSymlink != 0)
		assert.False(t, info.IsDir())

		info, err = os.Lstat(resolvedTarget)
		require.NoError(t, err)
		assert.False(t, info.IsDir())
	}

	t.Run("Absolute", func(t *testing.T) {
		link := filepath.Join(linkDir, "configfilelink")
		target := filepath.Join(testDir, "b", "configfiletarget")
		testSymlink(t, link, target, target)
	})
	t.Run("Relative", func(t *testing.T) {
		link := filepath.Join(linkDir, "configfilelink")
		target := filepath.Join("b", "c", "configfiletarget")
		resolvedTarget := filepath.Join(filepath.Dir(link), target)
		testSymlink(t, link, target, resolvedTarget)
	})
}
