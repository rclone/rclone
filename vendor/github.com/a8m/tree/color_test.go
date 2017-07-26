package tree

import (
	"os"
	"syscall"
	"testing"
)

var extsTests = []struct {
	name     string
	expected string
}{
	{"foo.jpg", "\x1b[1;35mfoo.jpg\x1b[0m"},
	{"bar.tar", "\x1b[1;31mbar.tar\x1b[0m"},
	{"baz.exe", "\x1b[1;32mbaz.exe\x1b[0m"},
}

func TestExtension(t *testing.T) {
	for _, test := range extsTests {
		fi := &file{name: test.name}
		no := &Node{FileInfo: fi}
		if actual := ANSIColor(no, fi.name); actual != test.expected {
			t.Errorf("\ngot:\n%+v\nexpected:\n%+v", actual, test.expected)
		}
	}
}

var modeTests = []struct {
	path     string
	name     string
	expected string
	mode     os.FileMode
}{
	{"", "simple", "simple", os.FileMode(0)},
	{"", "dir", "\x1b[1;34mdir\x1b[0m", os.ModeDir},
	{"", "socket", "\x1b[40;1;35msocket\x1b[0m", os.ModeSocket},
	{"", "fifo", "\x1b[40;33mfifo\x1b[0m", os.ModeNamedPipe},
	{"", "block", "\x1b[40;1;33mblock\x1b[0m", os.ModeDevice},
	{"", "char", "\x1b[40;1;33mchar\x1b[0m", os.ModeCharDevice},
	{"", "exist-symlink", "\x1b[1;36mexist-symlink\x1b[0m", os.ModeSymlink},
	{"fake-path-a8m", "fake-path", "\x1b[40;1;31mfake-path\x1b[0m", os.ModeSymlink},
	{"", "exec", "\x1b[1;32mexec\x1b[0m", os.FileMode(syscall.S_IXUSR)},
}

func TestFileMode(t *testing.T) {
	for _, test := range modeTests {
		fi := &file{name: test.name, mode: test.mode}
		no := &Node{FileInfo: fi, path: test.path}
		if actual := ANSIColor(no, fi.name); actual != test.expected {
			t.Errorf("\ngot:\n%+v\nexpected:\n%+v", actual, test.expected)
		}
	}
}
