package local

import (
	"runtime"
	"testing"

	"github.com/rclone/rclone/lib/encoder"
)

// Test Windows character replacements
var testsWindows = [][2]string{
	{`c:\temp`, `c:\temp`},
	{`\\?\UNC\theserver\dir\file.txt`, `\\?\UNC\theserver\dir\file.txt`},
	{`//?/UNC/theserver/dir\file.txt`, `\\?\UNC\theserver\dir\file.txt`},
	{`c:/temp`, `c:\temp`},
	{`C:/temp/file.txt`, `C:\temp\file.txt`},
	{`c:\!\"#¤%&/()=;:*^?+-`, `c:\!\＂#¤%&\()=;：＊^？+-`},
	{`c:\<>"|?*:&\<>"|?*:&\<>"|?*:&`, `c:\＜＞＂｜？＊：&\＜＞＂｜？＊：&\＜＞＂｜？＊：&`},
}

func TestCleanWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	for _, test := range testsWindows {
		got := cleanRootPath(test[0], true, encoder.OS)
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}

// Relative roots must stay relative so the OS resolves them against the
// live working directory rather than a canonicalised string that may no
// longer refer to the same directory (#9510).
var testsRelative = [][2]string{
	{".", "."},
	{"./", "."},
	{"sub/dir", "sub/dir"},
	{"sub/dir/", "sub/dir"},
	{"./sub/dir", "sub/dir"},
	{"sub/../dir", "dir"},
	{"..", ".."},
}

func TestCleanRootPathRelative(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skipf("non-windows only")
	}
	for _, test := range testsRelative {
		got := cleanRootPath(test[0], true, encoder.OS)
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}
