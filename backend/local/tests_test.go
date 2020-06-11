package local

import (
	"runtime"
	"testing"
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
		got := cleanRootPath(test[0], true, defaultEnc)
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}
