package local

import (
	"runtime"
	"testing"
)

var uncTestPaths = []string{
	`C:\Ba*d\P|a?t<h>\Windows\Folder`,
	`C:\Windows\Folder`,
	`\\?\C:\Windows\Folder`,
	`\\?\UNC\server\share\Desktop`,
	`\\?\unC\server\share\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\server\share\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`C:\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`C:\AbsoluteToRoot\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\server\share\Desktop`,
	`\\?\UNC\\share\folder\Desktop`,
	`\\server\share`,
}

var uncTestPathsResults = []string{
	`\\?\C:\Ba*d\P|a?t<h>\Windows\Folder`,
	`\\?\C:\Windows\Folder`,
	`\\?\C:\Windows\Folder`,
	`\\?\UNC\server\share\Desktop`,
	`\\?\unC\server\share\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\?\UNC\server\share\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\?\C:\Desktop\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\?\C:\AbsoluteToRoot\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path\Very Long path`,
	`\\?\UNC\server\share\Desktop`,
	`\\?\UNC\\share\folder\Desktop`,
	`\\?\UNC\server\share`,
}

// Test that UNC paths are converted.
func TestUncPaths(t *testing.T) {
	for i, p := range uncTestPaths {
		unc := uncPath(p)
		if unc != uncTestPathsResults[i] {
			t.Fatalf("UNC test path\nInput:%s\nOutput:%s\nExpected:%s", p, unc, uncTestPathsResults[i])
		}
		// Test we don't add more.
		unc = uncPath(unc)
		if unc != uncTestPathsResults[i] {
			t.Fatalf("UNC test path\nInput:%s\nOutput:%s\nExpected:%s", p, unc, uncTestPathsResults[i])
		}
	}
}

// Test Windows character replacements
var testsWindows = [][2]string{
	{`c:\temp`, `c:\temp`},
	{`\\?\UNC\theserver\dir\file.txt`, `\\?\UNC\theserver\dir\file.txt`},
	{`//?/UNC/theserver/dir\file.txt`, `\\?\UNC\theserver\dir\file.txt`},
	{`c:/temp`, `c:\temp`},
	{`/temp/file.txt`, `\temp\file.txt`},
	{`c:\!\"#¤%&/()=;:*^?+-`, `c:\!\＂#¤%&\()=;：＊^？+-`},
	{`c:\<>"|?*:&\<>"|?*:&\<>"|?*:&`, `c:\＜＞＂｜？＊：&\＜＞＂｜？＊：&\＜＞＂｜？＊：&`},
}

func TestCleanWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skipf("windows only")
	}
	for _, test := range testsWindows {
		got := cleanRootPath(test[0], true)
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}
