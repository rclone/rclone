package fspath

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	makeCorpus = flag.Bool("make-corpus", false, "Set to make the fuzzing corpus")
)

func TestCheckConfigName(t *testing.T) {
	for _, test := range []struct {
		in   string
		want error
	}{
		{"remote", nil},
		{"", errInvalidCharacters},
		{":remote:", errInvalidCharacters},
		{"remote:", errInvalidCharacters},
		{"rem:ote", errInvalidCharacters},
		{"rem/ote", errInvalidCharacters},
		{"rem\\ote", errInvalidCharacters},
		{"[remote", errInvalidCharacters},
		{"*", errInvalidCharacters},
		{"-remote", errCantStartWithDash},
		{"r-emote-", nil},
		{"_rem_ote_", nil},
	} {
		got := CheckConfigName(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestCheckRemoteName(t *testing.T) {
	for _, test := range []struct {
		in   string
		want error
	}{
		{":remote:", nil},
		{":s3:", nil},
		{"remote:", nil},
		{"", errInvalidCharacters},
		{"rem:ote", errInvalidCharacters},
		{"rem:ote:", errInvalidCharacters},
		{"remote", errInvalidCharacters},
		{"rem/ote:", errInvalidCharacters},
		{"rem\\ote:", errInvalidCharacters},
		{"[remote:", errInvalidCharacters},
		{"*:", errInvalidCharacters},
	} {
		got := checkRemoteName(test.in)
		assert.Equal(t, test.want, got, test.in)
	}
}

func TestParse(t *testing.T) {
	for testNumber, test := range []struct {
		in         string
		wantParsed Parsed
		wantErr    error
		win        bool // only run these tests on Windows
		noWin      bool // only run these tests on !Windows
	}{
		{
			in:      "",
			wantErr: errCantBeEmpty,
		}, {
			in:      ":",
			wantErr: errConfigName,
		}, {
			in:      "::",
			wantErr: errConfigNameEmpty,
		}, {
			in:      ":/:",
			wantErr: errInvalidCharacters,
		}, {
			in: "/:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "/:",
			},
		}, {
			in: "\\backslash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "/backslash:",
			},
			win: true,
		}, {
			in: "\\backslash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "\\backslash:",
			},
			noWin: true,
		}, {
			in: "/slash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "/slash:",
			},
		}, {
			in: "with\\backslash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "with/backslash:",
			},
			win: true,
		}, {
			in: "with\\backslash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "with\\backslash:",
			},
			noWin: true,
		}, {
			in: "with/slash:",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "with/slash:",
			},
		}, {
			in: "/path/to/file",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "/path/to/file",
			},
		}, {
			in: "/path:/to/file",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "/path:/to/file",
			},
		}, {
			in: "./path:/to/file",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "./path:/to/file",
			},
		}, {
			in: "./:colon.txt",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "./:colon.txt",
			},
		}, {
			in: "path/to/file",
			wantParsed: Parsed{
				ConfigString: "",
				Path:         "path/to/file",
			},
		}, {
			in: "remote:path/to/file",
			wantParsed: Parsed{
				ConfigString: "remote",
				Name:         "remote",
				Path:         "path/to/file",
			},
		}, {
			in:      "rem*ote:path/to/file",
			wantErr: errInvalidCharacters,
		}, {
			in: "remote:/path/to/file",
			wantParsed: Parsed{
				ConfigString: "remote",
				Name:         "remote",
				Path:         "/path/to/file",
			},
		}, {
			in:      "rem.ote:/path/to/file",
			wantErr: errInvalidCharacters,
		}, {
			in: ":backend:/path/to/file",
			wantParsed: Parsed{
				ConfigString: ":backend",
				Name:         ":backend",
				Path:         "/path/to/file",
			},
		}, {
			in:      ":bac*kend:/path/to/file",
			wantErr: errInvalidCharacters,
		}, {
			in: `C:\path\to\file`,
			wantParsed: Parsed{
				Name: "",
				Path: `C:/path/to/file`,
			},
			win: true,
		}, {
			in: `C:\path\to\file`,
			wantParsed: Parsed{
				Name:         "C",
				ConfigString: "C",
				Path:         `\path\to\file`,
			},
			noWin: true,
		}, {
			in: `\path\to\file`,
			wantParsed: Parsed{
				Name: "",
				Path: `/path/to/file`,
			},
			win: true,
		}, {
			in: `\path\to\file`,
			wantParsed: Parsed{
				Name: "",
				Path: `\path\to\file`,
			},
			noWin: true,
		}, {
			in: `remote:\path\to\file`,
			wantParsed: Parsed{
				Name:         "remote",
				ConfigString: "remote",
				Path:         `/path/to/file`,
			},
			win: true,
		}, {
			in: `remote:\path\to\file`,
			wantParsed: Parsed{
				Name:         "remote",
				ConfigString: "remote",
				Path:         `\path\to\file`,
			},
			noWin: true,
		}, {
			in: `D:/path/to/file`,
			wantParsed: Parsed{
				Name: "",
				Path: `D:/path/to/file`,
			},
			win: true,
		}, {
			in: `D:/path/to/file`,
			wantParsed: Parsed{
				Name:         "D",
				ConfigString: "D",
				Path:         `/path/to/file`,
			},
			noWin: true,
		}, {
			in: `:backend,param1:/path/to/file`,
			wantParsed: Parsed{
				ConfigString: `:backend,param1`,
				Name:         ":backend",
				Path:         "/path/to/file",
				Config: configmap.Simple{
					"param1": "true",
				},
			},
		}, {
			in: `:backend,param1=value:/path/to/file`,
			wantParsed: Parsed{
				ConfigString: `:backend,param1=value`,
				Name:         ":backend",
				Path:         "/path/to/file",
				Config: configmap.Simple{
					"param1": "value",
				},
			},
		}, {
			in: `:backend,param1=value1,param2,param3=value3:/path/to/file`,
			wantParsed: Parsed{
				ConfigString: `:backend,param1=value1,param2,param3=value3`,
				Name:         ":backend",
				Path:         "/path/to/file",
				Config: configmap.Simple{
					"param1": "value1",
					"param2": "true",
					"param3": "value3",
				},
			},
		}, {
			in: `:backend,param1=value1,param2="value2",param3='value3':/path/to/file`,
			wantParsed: Parsed{
				ConfigString: `:backend,param1=value1,param2="value2",param3='value3'`,
				Name:         ":backend",
				Path:         "/path/to/file",
				Config: configmap.Simple{
					"param1": "value1",
					"param2": "value2",
					"param3": "value3",
				},
			},
		}, {
			in:      `:backend,param-1=value:/path/to/file`,
			wantErr: errBadConfigParam,
		}, {
			in:      `:backend,param1="value"x:/path/to/file`,
			wantErr: errAfterQuote,
		}, {
			in:      `:backend,`,
			wantErr: errParam,
		}, {
			in:      `:backend,param=value`,
			wantErr: errValue,
		}, {
			in:      `:backend,param="value'`,
			wantErr: errQuotedValue,
		}, {
			in:      `:backend,param1="value"`,
			wantErr: errAfterQuote,
		}, {
			in:      `:backend,=value:`,
			wantErr: errEmptyConfigParam,
		}, {
			in:      `:backend,:`,
			wantErr: errEmptyConfigParam,
		}, {
			in:      `:backend,,:`,
			wantErr: errEmptyConfigParam,
		}, {
			in: `:backend,param=:path`,
			wantParsed: Parsed{
				ConfigString: `:backend,param=`,
				Name:         ":backend",
				Path:         "path",
				Config: configmap.Simple{
					"param": "",
				},
			},
		}, {
			in: `:backend,param="with""quote":path`,
			wantParsed: Parsed{
				ConfigString: `:backend,param="with""quote"`,
				Name:         ":backend",
				Path:         "path",
				Config: configmap.Simple{
					"param": `with"quote`,
				},
			},
		}, {
			in: `:backend,param='''''':`,
			wantParsed: Parsed{
				ConfigString: `:backend,param=''''''`,
				Name:         ":backend",
				Path:         "",
				Config: configmap.Simple{
					"param": `''`,
				},
			},
		}, {
			in:      `:backend,param=''bad'':`,
			wantErr: errAfterQuote,
		},
	} {
		gotParsed, gotErr := Parse(test.in)
		if runtime.GOOS == "windows" && test.noWin {
			continue
		}
		if runtime.GOOS != "windows" && test.win {
			continue
		}
		assert.Equal(t, test.wantErr, gotErr, test.in)
		if test.wantErr == nil {
			assert.Equal(t, test.wantParsed, gotParsed, test.in)
		}
		if *makeCorpus {
			// write the test corpus for fuzzing
			require.NoError(t, os.MkdirAll("corpus", 0777))
			require.NoError(t, ioutil.WriteFile(fmt.Sprintf("corpus/%02d", testNumber), []byte(test.in), 0666))
		}

	}
}

func TestSplitFs(t *testing.T) {
	for _, test := range []struct {
		remote, wantRemoteName, wantRemotePath string
		wantErr                                error
	}{
		{"", "", "", errCantBeEmpty},

		{"remote:", "remote:", "", nil},
		{"remote:potato", "remote:", "potato", nil},
		{"remote:/", "remote:", "/", nil},
		{"remote:/potato", "remote:", "/potato", nil},
		{"remote:/potato/potato", "remote:", "/potato/potato", nil},
		{"remote:potato/sausage", "remote:", "potato/sausage", nil},
		{"rem.ote:potato/sausage", "", "", errInvalidCharacters},

		{":remote:", ":remote:", "", nil},
		{":remote:potato", ":remote:", "potato", nil},
		{":remote:/", ":remote:", "/", nil},
		{":remote:/potato", ":remote:", "/potato", nil},
		{":remote:/potato/potato", ":remote:", "/potato/potato", nil},
		{":remote:potato/sausage", ":remote:", "potato/sausage", nil},
		{":rem[ote:potato/sausage", "", "", errInvalidCharacters},

		{"/", "", "/", nil},
		{"/root", "", "/root", nil},
		{"/a/b", "", "/a/b", nil},
		{"root", "", "root", nil},
		{"a/b", "", "a/b", nil},
		{"root/", "", "root/", nil},
		{"a/b/", "", "a/b/", nil},
	} {
		gotRemoteName, gotRemotePath, gotErr := SplitFs(test.remote)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.wantRemoteName, gotRemoteName, test.remote)
		assert.Equal(t, test.wantRemotePath, gotRemotePath, test.remote)
		if gotErr == nil {
			assert.Equal(t, test.remote, gotRemoteName+gotRemotePath, fmt.Sprintf("%s: %q + %q != %q", test.remote, gotRemoteName, gotRemotePath, test.remote))
		}
	}
}

func TestSplit(t *testing.T) {
	for _, test := range []struct {
		remote, wantParent, wantLeaf string
		wantErr                      error
	}{
		{"", "", "", errCantBeEmpty},

		{"remote:", "remote:", "", nil},
		{"remote:potato", "remote:", "potato", nil},
		{"remote:/", "remote:/", "", nil},
		{"remote:/potato", "remote:/", "potato", nil},
		{"remote:/potato/potato", "remote:/potato/", "potato", nil},
		{"remote:potato/sausage", "remote:potato/", "sausage", nil},
		{"rem.ote:potato/sausage", "", "", errInvalidCharacters},

		{":remote:", ":remote:", "", nil},
		{":remote:potato", ":remote:", "potato", nil},
		{":remote:/", ":remote:/", "", nil},
		{":remote:/potato", ":remote:/", "potato", nil},
		{":remote:/potato/potato", ":remote:/potato/", "potato", nil},
		{":remote:potato/sausage", ":remote:potato/", "sausage", nil},
		{":rem[ote:potato/sausage", "", "", errInvalidCharacters},

		{"/", "/", "", nil},
		{"/root", "/", "root", nil},
		{"/a/b", "/a/", "b", nil},
		{"root", "", "root", nil},
		{"a/b", "a/", "b", nil},
		{"root/", "root/", "", nil},
		{"a/b/", "a/b/", "", nil},
	} {
		gotParent, gotLeaf, gotErr := Split(test.remote)
		assert.Equal(t, test.wantErr, gotErr)
		assert.Equal(t, test.wantParent, gotParent, test.remote)
		assert.Equal(t, test.wantLeaf, gotLeaf, test.remote)
		if gotErr == nil {
			assert.Equal(t, test.remote, gotParent+gotLeaf, fmt.Sprintf("%s: %q + %q != %q", test.remote, gotParent, gotLeaf, test.remote))
		}
	}
}

func TestMakeAbsolute(t *testing.T) {
	for _, test := range []struct {
		in   string
		want string
	}{
		{"", ""},
		{".", ""},
		{"/.", "/"},
		{"../potato", "potato"},
		{"/../potato", "/potato"},
		{"./../potato", "potato"},
		{"//../potato", "/potato"},
		{"././../potato", "potato"},
		{"././potato/../../onion", "onion"},
	} {
		got := makeAbsolute(test.in)
		assert.Equal(t, test.want, got, test)
	}
}

func TestJoinRootPath(t *testing.T) {
	for _, test := range []struct {
		remote   string
		filePath string
		want     string
	}{
		{"", "", ""},
		{"", "/", "/"},
		{"/", "", "/"},
		{"/", "/", "/"},
		{"/", "//", "/"},
		{"/root", "", "/root"},
		{"/root", "/", "/root"},
		{"/root", "//", "/root"},
		{"/a/b", "", "/a/b"},
		{"//", "/", "//"},
		{"//server", "path", "//server/path"},
		{"//server/sub", "path", "//server/sub/path"},
		{"//server", "//path", "//server/path"},
		{"//server/sub", "//path", "//server/sub/path"},
		{"//", "/", "//"},
		{"//server", "path", "//server/path"},
		{"//server/sub", "path", "//server/sub/path"},
		{"//server", "//path", "//server/path"},
		{"//server/sub", "//path", "//server/sub/path"},
		{filepath.FromSlash("//server/sub"), filepath.FromSlash("//path"), "//server/sub/path"},
		{"s3:", "", "s3:"},
		{"s3:", ".", "s3:"},
		{"s3:.", ".", "s3:"},
		{"s3:", "..", "s3:"},
		{"s3:dir", "sub", "s3:dir/sub"},
		{"s3:dir", "/sub", "s3:dir/sub"},
		{"s3:dir", "./sub", "s3:dir/sub"},
		{"s3:/dir", "/sub/", "s3:/dir/sub"},
		{"s3:dir", "..", "s3:dir"},
		{"s3:dir", "/..", "s3:dir"},
		{"s3:dir", "/../", "s3:dir"},
	} {
		got := JoinRootPath(test.remote, test.filePath)
		assert.Equal(t, test.want, got, test)
	}
}
