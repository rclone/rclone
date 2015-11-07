package local

import (
	"testing"
)

var uncTestPaths = []string{
	"C:\\Ba*d\\P|a?t<h>\\Windows\\Folder",
	"C:/Ba*d/P|a?t<h>/Windows\\Folder",
	"C:\\Windows\\Folder",
	"\\\\?\\C:\\Windows\\Folder",
	"//?/C:/Windows/Folder",
	"\\\\?\\UNC\\server\\share\\Desktop",
	"\\\\?\\unC\\server\\share\\Desktop\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path",
	"\\\\server\\share\\Desktop\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path",
	"C:\\Desktop\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path",
	"C:\\AbsoluteToRoot\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path\\Very Long path",
	"\\\\server\\share\\Desktop",
	"\\\\?\\UNC\\\\share\\folder\\Desktop",
	"\\\\server\\share",
}

var uncTestPathsResults = []string{
	`\\?\C:\Ba*d\P|a?t<h>\Windows\Folder`,
	`\\?\C:\Ba*d\P|a?t<h>\Windows\Folder`,
	`\\?\C:\Windows\Folder`,
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

var utf8Tests = [][2]string{
	[2]string{"ABC", "ABC"},
	[2]string{string([]byte{0x80}), "�"},
	[2]string{string([]byte{'a', 0x80, 'b'}), "a�b"},
}

func TestCleanUtf8(t *testing.T) {
	f := &Fs{}
	f.warned = make(map[string]struct{})
	for _, test := range utf8Tests {
		got := f.cleanUtf8(test[0])
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}

// Test Windows character replacements
var testsWindows = [][2]string{
	[2]string{`c:\temp`, `c:\temp`},
	[2]string{`\\?\UNC\theserver\dir\file.txt`, `\\?\UNC\theserver\dir\file.txt`},
	[2]string{`//?/UNC/theserver/dir\file.txt`, `//?/UNC/theserver/dir\file.txt`},
	[2]string{"c:/temp", "c:/temp"},
	[2]string{"/temp/file.txt", "/temp/file.txt"},
	[2]string{`!\"#¤%&/()=;:*^?+-`, "!\\_#¤%&/()=;__^_+-"},
	[2]string{`<>"|?*:&\<>"|?*:&\<>"|?*:&`, "_______&\\_______&\\_______&"},
}

func TestCleanWindows(t *testing.T) {
	for _, test := range testsWindows {
		got := cleanWindowsName(nil, test[0])
		expect := test[1]
		if got != expect {
			t.Fatalf("got %q, expected %q", got, expect)
		}
	}
}
