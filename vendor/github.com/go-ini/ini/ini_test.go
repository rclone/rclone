// Copyright 2014 Unknwon
//
// Licensed under the Apache License, Version 2.0 (the "License"): you may
// not use this file except in compliance with the License. You may obtain
// a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
// License for the specific language governing permissions and limitations
// under the License.

package ini_test

import (
	"bytes"
	"flag"
	"io/ioutil"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/ini.v1"
)

const (
	_CONF_DATA = `
	; Package name
	NAME        = ini
	; Package version
	VERSION     = v1
	; Package import path
	IMPORT_PATH = gopkg.in/%(NAME)s.%(VERSION)s
	
	# Information about package author
	# Bio can be written in multiple lines.
	[author]
	NAME   = Unknwon  ; Succeeding comment
	E-MAIL = fake@localhost
	GITHUB = https://github.com/%(NAME)s
	BIO    = """Gopher.
	Coding addict.
	Good man.
	"""  # Succeeding comment`
	_MINIMAL_CONF   = "testdata/minimal.ini"
	_FULL_CONF      = "testdata/full.ini"
	_NOT_FOUND_CONF = "testdata/404.ini"
)

var update = flag.Bool("update", false, "Update .golden files")

func TestLoad(t *testing.T) {
	Convey("Load from good data sources", t, func() {
		f, err := ini.Load([]byte(`
NAME = ini
VERSION = v1
IMPORT_PATH = gopkg.in/%(NAME)s.%(VERSION)s`),
			"testdata/minimal.ini",
			ioutil.NopCloser(bytes.NewReader([]byte(`
[author]
NAME = Unknwon
`))),
		)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		// Validate values make sure all sources are loaded correctly
		sec := f.Section("")
		So(sec.Key("NAME").String(), ShouldEqual, "ini")
		So(sec.Key("VERSION").String(), ShouldEqual, "v1")
		So(sec.Key("IMPORT_PATH").String(), ShouldEqual, "gopkg.in/ini.v1")

		sec = f.Section("author")
		So(sec.Key("NAME").String(), ShouldEqual, "Unknwon")
		So(sec.Key("E-MAIL").String(), ShouldEqual, "u@gogs.io")
	})

	Convey("Load from bad data sources", t, func() {
		Convey("Invalid input", func() {
			_, err := ini.Load(_NOT_FOUND_CONF)
			So(err, ShouldNotBeNil)
		})

		Convey("Unsupported type", func() {
			_, err := ini.Load(123)
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Can't properly parse INI files containing `#` or `;` in value", t, func() {
		f, err := ini.Load([]byte(`
	[author]
	NAME = U#n#k#n#w#o#n
	GITHUB = U;n;k;n;w;o;n
	`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		sec := f.Section("author")
		nameValue := sec.Key("NAME").String()
		githubValue := sec.Key("GITHUB").String()
		So(nameValue, ShouldEqual, "U")
		So(githubValue, ShouldEqual, "U")
	})

	Convey("Can't parse small python-compatible INI files", t, func() {
		f, err := ini.Load([]byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   foo
   bar
   foobar
   barfoo
   -----END RSA PRIVATE KEY-----
`))
		So(err, ShouldNotBeNil)
		So(f, ShouldBeNil)
		So(err.Error(), ShouldEqual, "key-value delimiter not found: foo\n")
	})

	Convey("Can't parse big python-compatible INI files", t, func() {
		f, err := ini.Load([]byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   1foo
   2bar
   3foobar
   4barfoo
   5foo
   6bar
   7foobar
   8barfoo
   9foo
   10bar
   11foobar
   12barfoo
   13foo
   14bar
   15foobar
   16barfoo
   17foo
   18bar
   19foobar
   20barfoo
   21foo
   22bar
   23foobar
   24barfoo
   25foo
   26bar
   27foobar
   28barfoo
   29foo
   30bar
   31foobar
   32barfoo
   33foo
   34bar
   35foobar
   36barfoo
   37foo
   38bar
   39foobar
   40barfoo
   41foo
   42bar
   43foobar
   44barfoo
   45foo
   46bar
   47foobar
   48barfoo
   49foo
   50bar
   51foobar
   52barfoo
   53foo
   54bar
   55foobar
   56barfoo
   57foo
   58bar
   59foobar
   60barfoo
   61foo
   62bar
   63foobar
   64barfoo
   65foo
   66bar
   67foobar
   68barfoo
   69foo
   70bar
   71foobar
   72barfoo
   73foo
   74bar
   75foobar
   76barfoo
   77foo
   78bar
   79foobar
   80barfoo
   81foo
   82bar
   83foobar
   84barfoo
   85foo
   86bar
   87foobar
   88barfoo
   89foo
   90bar
   91foobar
   92barfoo
   93foo
   94bar
   95foobar
   96barfoo
   -----END RSA PRIVATE KEY-----
`))
		So(err, ShouldNotBeNil)
		So(f, ShouldBeNil)
		So(err.Error(), ShouldEqual, "key-value delimiter not found: 1foo\n")
	})
}

func TestLooseLoad(t *testing.T) {
	Convey("Load from data sources with option `Loose` true", t, func() {
		f, err := ini.LoadSources(ini.LoadOptions{Loose: true}, _NOT_FOUND_CONF, _MINIMAL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		Convey("Inverse case", func() {
			_, err = ini.Load(_NOT_FOUND_CONF)
			So(err, ShouldNotBeNil)
		})
	})
}

func TestInsensitiveLoad(t *testing.T) {
	Convey("Insensitive to section and key names", t, func() {
		f, err := ini.InsensitiveLoad(_MINIMAL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		So(f.Section("Author").Key("e-mail").String(), ShouldEqual, "u@gogs.io")

		Convey("Write out", func() {
			var buf bytes.Buffer
			_, err := f.WriteTo(&buf)
			So(err, ShouldBeNil)
			So(buf.String(), ShouldEqual, `[author]
e-mail = u@gogs.io

`)
		})

		Convey("Inverse case", func() {
			f, err := ini.Load(_MINIMAL_CONF)
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			So(f.Section("Author").Key("e-mail").String(), ShouldBeEmpty)
		})
	})
}

func TestLoadSources(t *testing.T) {
	Convey("Load from data sources with options", t, func() {
		Convey("with true `AllowPythonMultilineValues`", func() {
			Convey("Ignore nonexistent files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true, Loose: true}, _NOT_FOUND_CONF, _MINIMAL_CONF)
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				Convey("Inverse case", func() {
					_, err = ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, _NOT_FOUND_CONF)
					So(err, ShouldNotBeNil)
				})
			})

			Convey("Insensitive to section and key names", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true, Insensitive: true}, _MINIMAL_CONF)
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("Author").Key("e-mail").String(), ShouldEqual, "u@gogs.io")

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `[author]
e-mail = u@gogs.io

`)
				})

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, _MINIMAL_CONF)
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("Author").Key("e-mail").String(), ShouldBeEmpty)
				})
			})

			Convey("Ignore continuation lines", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					IgnoreContinuation:         true,
				}, []byte(`
key1=a\b\
key2=c\d\
key3=value`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key1").String(), ShouldEqual, `a\b\`)
				So(f.Section("").Key("key2").String(), ShouldEqual, `c\d\`)
				So(f.Section("").Key("key3").String(), ShouldEqual, "value")

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=a\b\
key2=c\d\`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("key1").String(), ShouldEqual, `a\bkey2=c\d`)
				})
			})

			Convey("Ignore inline comments", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					IgnoreInlineComment:        true,
				}, []byte(`
key1=value ;comment
key2=value2 #comment2
key3=val#ue #comment3`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key1").String(), ShouldEqual, `value ;comment`)
				So(f.Section("").Key("key2").String(), ShouldEqual, `value2 #comment2`)
				So(f.Section("").Key("key3").String(), ShouldEqual, `val#ue #comment3`)

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=value ;comment
key2=value2 #comment2`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("key1").String(), ShouldEqual, `value`)
					So(f.Section("").Key("key1").Comment, ShouldEqual, `;comment`)
					So(f.Section("").Key("key2").String(), ShouldEqual, `value2`)
					So(f.Section("").Key("key2").Comment, ShouldEqual, `#comment2`)
				})
			})

			Convey("Allow boolean type keys", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					AllowBooleanKeys:           true,
				}, []byte(`
key1=hello
#key2
key3`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").KeyStrings(), ShouldResemble, []string{"key1", "key3"})
				So(f.Section("").Key("key3").MustBool(false), ShouldBeTrue)

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `key1 = hello
# key2
key3
`)
				})

				Convey("Inverse case", func() {
					_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
key1=hello
#key2
key3`))
					So(err, ShouldNotBeNil)
				})
			})

			Convey("Allow shadow keys", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowShadows: true, AllowPythonMultilineValues: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section(`remote "origin"`).Key("url").String(), ShouldEqual, "https://github.com/Antergone/test1.git")
				So(f.Section(`remote "origin"`).Key("url").ValueWithShadows(), ShouldResemble, []string{
					"https://github.com/Antergone/test1.git",
					"https://github.com/Antergone/test2.git",
				})
				So(f.Section(`remote "origin"`).Key("fetch").String(), ShouldEqual, "+refs/heads/*:refs/remotes/origin/*")

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `[remote "origin"]
url   = https://github.com/Antergone/test1.git
url   = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*

`)
				})

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section(`remote "origin"`).Key("url").String(), ShouldEqual, "https://github.com/Antergone/test2.git")
				})
			})

			Convey("Unescape double quotes inside value", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					UnescapeValueDoubleQuotes:  true,
				}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("create_repo").String(), ShouldEqual, `创建了仓库 <a href="%s">%s</a>`)

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("create_repo").String(), ShouldEqual, `"创建了仓库 <a href=\"%s\">%s</a>"`)
				})
			})

			Convey("Unescape comment symbols inside value", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues:  true,
					IgnoreInlineComment:         true,
					UnescapeValueCommentSymbols: true,
				}, []byte(`
key = test value <span style="color: %s\; background: %s">more text</span>
`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key").String(), ShouldEqual, `test value <span style="color: %s; background: %s">more text</span>`)
			})

			Convey("Can parse small python-compatible INI files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					Insensitive:                true,
					UnparseableSections:        []string{"core_lesson", "comments"},
				}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  foo
  bar
  foobar
  barfoo
  -----END RSA PRIVATE KEY-----
`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("long").Key("long_rsa_private_key").String(), ShouldEqual, "-----BEGIN RSA PRIVATE KEY-----\nfoo\nbar\nfoobar\nbarfoo\n-----END RSA PRIVATE KEY-----")
			})

			Convey("Can parse big python-compatible INI files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					Insensitive:                true,
					UnparseableSections:        []string{"core_lesson", "comments"},
				}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
   1foo
   2bar
   3foobar
   4barfoo
   5foo
   6bar
   7foobar
   8barfoo
   9foo
   10bar
   11foobar
   12barfoo
   13foo
   14bar
   15foobar
   16barfoo
   17foo
   18bar
   19foobar
   20barfoo
   21foo
   22bar
   23foobar
   24barfoo
   25foo
   26bar
   27foobar
   28barfoo
   29foo
   30bar
   31foobar
   32barfoo
   33foo
   34bar
   35foobar
   36barfoo
   37foo
   38bar
   39foobar
   40barfoo
   41foo
   42bar
   43foobar
   44barfoo
   45foo
   46bar
   47foobar
   48barfoo
   49foo
   50bar
   51foobar
   52barfoo
   53foo
   54bar
   55foobar
   56barfoo
   57foo
   58bar
   59foobar
   60barfoo
   61foo
   62bar
   63foobar
   64barfoo
   65foo
   66bar
   67foobar
   68barfoo
   69foo
   70bar
   71foobar
   72barfoo
   73foo
   74bar
   75foobar
   76barfoo
   77foo
   78bar
   79foobar
   80barfoo
   81foo
   82bar
   83foobar
   84barfoo
   85foo
   86bar
   87foobar
   88barfoo
   89foo
   90bar
   91foobar
   92barfoo
   93foo
   94bar
   95foobar
   96barfoo
   -----END RSA PRIVATE KEY-----
`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("long").Key("long_rsa_private_key").String(), ShouldEqual, `-----BEGIN RSA PRIVATE KEY-----
1foo
2bar
3foobar
4barfoo
5foo
6bar
7foobar
8barfoo
9foo
10bar
11foobar
12barfoo
13foo
14bar
15foobar
16barfoo
17foo
18bar
19foobar
20barfoo
21foo
22bar
23foobar
24barfoo
25foo
26bar
27foobar
28barfoo
29foo
30bar
31foobar
32barfoo
33foo
34bar
35foobar
36barfoo
37foo
38bar
39foobar
40barfoo
41foo
42bar
43foobar
44barfoo
45foo
46bar
47foobar
48barfoo
49foo
50bar
51foobar
52barfoo
53foo
54bar
55foobar
56barfoo
57foo
58bar
59foobar
60barfoo
61foo
62bar
63foobar
64barfoo
65foo
66bar
67foobar
68barfoo
69foo
70bar
71foobar
72barfoo
73foo
74bar
75foobar
76barfoo
77foo
78bar
79foobar
80barfoo
81foo
82bar
83foobar
84barfoo
85foo
86bar
87foobar
88barfoo
89foo
90bar
91foobar
92barfoo
93foo
94bar
95foobar
96barfoo
-----END RSA PRIVATE KEY-----`)
			})

			Convey("Allow unparsable sections", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: true,
					Insensitive:                true,
					UnparseableSections:        []string{"core_lesson", "comments"},
				}, []byte(`
Lesson_Location = 87
Lesson_Status = C
Score = 3
Time = 00:02:30

[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[COMMENTS]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("score").String(), ShouldEqual, "3")
				So(f.Section("").Body(), ShouldBeEmpty)
				So(f.Section("core_lesson").Body(), ShouldEqual, `my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`)
				So(f.Section("comments").Body(), ShouldEqual, `<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`)

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `lesson_location = 87
lesson_status   = C
score           = 3
time            = 00:02:30

[core_lesson]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[comments]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>
`)
				})

				Convey("Inverse case", func() {
					_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: true}, []byte(`
[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`))
					So(err, ShouldNotBeNil)
				})
			})

			Convey("And false `SpaceBeforeInlineComment`", func() {
				Convey("Can't parse INI files containing `#` or `;` in value", func() {
					f, err := ini.LoadSources(
						ini.LoadOptions{AllowPythonMultilineValues: false, SpaceBeforeInlineComment: false},
						[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)
						sec := f.Section("author")
					nameValue := sec.Key("NAME").String()
					githubValue := sec.Key("GITHUB").String()
					So(nameValue, ShouldEqual, "U")
					So(githubValue, ShouldEqual, "U")
				})
			})

			Convey("And true `SpaceBeforeInlineComment`", func() {
				Convey("Can parse INI files containing `#` or `;` in value", func() {
					f, err := ini.LoadSources(
						ini.LoadOptions{AllowPythonMultilineValues: false, SpaceBeforeInlineComment: true},
						[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)
					sec := f.Section("author")
					nameValue := sec.Key("NAME").String()
					githubValue := sec.Key("GITHUB").String()
					So(nameValue, ShouldEqual, "U#n#k#n#w#o#n")
					So(githubValue, ShouldEqual, "U;n;k;n;w;o;n")
				})
			})
		})

		Convey("with false `AllowPythonMultilineValues`", func() {
			Convey("Ignore nonexistent files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false, Loose: true}, _NOT_FOUND_CONF, _MINIMAL_CONF)
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				Convey("Inverse case", func() {
					_, err = ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, _NOT_FOUND_CONF)
					So(err, ShouldNotBeNil)
				})
			})

			Convey("Insensitive to section and key names", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false, Insensitive: true}, _MINIMAL_CONF)
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("Author").Key("e-mail").String(), ShouldEqual, "u@gogs.io")

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `[author]
e-mail = u@gogs.io

`)
				})

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, _MINIMAL_CONF)
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("Author").Key("e-mail").String(), ShouldBeEmpty)
				})
			})

			Convey("Ignore continuation lines", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
					IgnoreContinuation:         true,
				}, []byte(`
key1=a\b\
key2=c\d\
key3=value`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key1").String(), ShouldEqual, `a\b\`)
				So(f.Section("").Key("key2").String(), ShouldEqual, `c\d\`)
				So(f.Section("").Key("key3").String(), ShouldEqual, "value")

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=a\b\
key2=c\d\`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("key1").String(), ShouldEqual, `a\bkey2=c\d`)
				})
			})

			Convey("Ignore inline comments", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
					IgnoreInlineComment:        true,
				}, []byte(`
key1=value ;comment
key2=value2 #comment2
key3=val#ue #comment3`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key1").String(), ShouldEqual, `value ;comment`)
				So(f.Section("").Key("key2").String(), ShouldEqual, `value2 #comment2`)
				So(f.Section("").Key("key3").String(), ShouldEqual, `val#ue #comment3`)

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=value ;comment
key2=value2 #comment2`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("key1").String(), ShouldEqual, `value`)
					So(f.Section("").Key("key1").Comment, ShouldEqual, `;comment`)
					So(f.Section("").Key("key2").String(), ShouldEqual, `value2`)
					So(f.Section("").Key("key2").Comment, ShouldEqual, `#comment2`)
				})
			})

			Convey("Allow boolean type keys", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
					AllowBooleanKeys:           true,
				}, []byte(`
key1=hello
#key2
key3`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").KeyStrings(), ShouldResemble, []string{"key1", "key3"})
				So(f.Section("").Key("key3").MustBool(false), ShouldBeTrue)

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `key1 = hello
# key2
key3
`)
				})

				Convey("Inverse case", func() {
					_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
key1=hello
#key2
key3`))
					So(err, ShouldNotBeNil)
				})
			})

			Convey("Allow shadow keys", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false, AllowShadows: true}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section(`remote "origin"`).Key("url").String(), ShouldEqual, "https://github.com/Antergone/test1.git")
				So(f.Section(`remote "origin"`).Key("url").ValueWithShadows(), ShouldResemble, []string{
					"https://github.com/Antergone/test1.git",
					"https://github.com/Antergone/test2.git",
				})
				So(f.Section(`remote "origin"`).Key("fetch").String(), ShouldEqual, "+refs/heads/*:refs/remotes/origin/*")

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `[remote "origin"]
url   = https://github.com/Antergone/test1.git
url   = https://github.com/Antergone/test2.git
fetch = +refs/heads/*:refs/remotes/origin/*

`)
				})

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[remote "origin"]
url = https://github.com/Antergone/test1.git
url = https://github.com/Antergone/test2.git`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section(`remote "origin"`).Key("url").String(), ShouldEqual, "https://github.com/Antergone/test2.git")
				})
			})

			Convey("Unescape double quotes inside value", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
					UnescapeValueDoubleQuotes:  true,
				}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("create_repo").String(), ShouldEqual, `创建了仓库 <a href="%s">%s</a>`)

				Convey("Inverse case", func() {
					f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)

					So(f.Section("").Key("create_repo").String(), ShouldEqual, `"创建了仓库 <a href=\"%s\">%s</a>"`)
				})
			})

			Convey("Unescape comment symbols inside value", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues:  false,
					IgnoreInlineComment:         true,
					UnescapeValueCommentSymbols: true,
				}, []byte(`
key = test value <span style="color: %s\; background: %s">more text</span>
`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key").String(), ShouldEqual, `test value <span style="color: %s; background: %s">more text</span>`)
			})

			Convey("Can't parse small python-compatible INI files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  foo
  bar
  foobar
  barfoo
  -----END RSA PRIVATE KEY-----
`))
				So(err, ShouldNotBeNil)
				So(f, ShouldBeNil)
				So(err.Error(), ShouldEqual, "key-value delimiter not found: foo\n")
			})

			Convey("Can't parse big python-compatible INI files", func() {
				f, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[long]
long_rsa_private_key = -----BEGIN RSA PRIVATE KEY-----
  1foo
  2bar
  3foobar
  4barfoo
  5foo
  6bar
  7foobar
  8barfoo
  9foo
  10bar
  11foobar
  12barfoo
  13foo
  14bar
  15foobar
  16barfoo
  17foo
  18bar
  19foobar
  20barfoo
  21foo
  22bar
  23foobar
  24barfoo
  25foo
  26bar
  27foobar
  28barfoo
  29foo
  30bar
  31foobar
  32barfoo
  33foo
  34bar
  35foobar
  36barfoo
  37foo
  38bar
  39foobar
  40barfoo
  41foo
  42bar
  43foobar
  44barfoo
  45foo
  46bar
  47foobar
  48barfoo
  49foo
  50bar
  51foobar
  52barfoo
  53foo
  54bar
  55foobar
  56barfoo
  57foo
  58bar
  59foobar
  60barfoo
  61foo
  62bar
  63foobar
  64barfoo
  65foo
  66bar
  67foobar
  68barfoo
  69foo
  70bar
  71foobar
  72barfoo
  73foo
  74bar
  75foobar
  76barfoo
  77foo
  78bar
  79foobar
  80barfoo
  81foo
  82bar
  83foobar
  84barfoo
  85foo
  86bar
  87foobar
  88barfoo
  89foo
  90bar
  91foobar
  92barfoo
  93foo
  94bar
  95foobar
  96barfoo
  -----END RSA PRIVATE KEY-----
`))
				So(err, ShouldNotBeNil)
				So(f, ShouldBeNil)
				So(err.Error(), ShouldEqual, "key-value delimiter not found: 1foo\n")
			})

			Convey("Allow unparsable sections", func() {
				f, err := ini.LoadSources(ini.LoadOptions{
					AllowPythonMultilineValues: false,
					Insensitive:                true,
					UnparseableSections:        []string{"core_lesson", "comments"},
				}, []byte(`
Lesson_Location = 87
Lesson_Status = C
Score = 3
Time = 00:02:30

[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[COMMENTS]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("score").String(), ShouldEqual, "3")
				So(f.Section("").Body(), ShouldBeEmpty)
				So(f.Section("core_lesson").Body(), ShouldEqual, `my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`)
				So(f.Section("comments").Body(), ShouldEqual, `<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>`)

				Convey("Write out", func() {
					var buf bytes.Buffer
					_, err := f.WriteTo(&buf)
					So(err, ShouldBeNil)
					So(buf.String(), ShouldEqual, `lesson_location = 87
lesson_status   = C
score           = 3
time            = 00:02:30

[core_lesson]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data

[comments]
<1><L.Slide#2> This slide has the fuel listed in the wrong units <e.1>
`)
				})

				Convey("Inverse case", func() {
					_, err := ini.LoadSources(ini.LoadOptions{AllowPythonMultilineValues: false}, []byte(`
[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`))
					So(err, ShouldNotBeNil)
				})
			})

			Convey("And false `SpaceBeforeInlineComment`", func() {
				Convey("Can't parse INI files containing `#` or `;` in value", func() {
					f, err := ini.LoadSources(
						ini.LoadOptions{AllowPythonMultilineValues: true, SpaceBeforeInlineComment: false},
						[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)
					sec := f.Section("author")
					nameValue := sec.Key("NAME").String()
					githubValue := sec.Key("GITHUB").String()
					So(nameValue, ShouldEqual, "U")
					So(githubValue, ShouldEqual, "U")
				})
			})

			Convey("And true `SpaceBeforeInlineComment`", func() {
				Convey("Can parse INI files containing `#` or `;` in value", func() {
					f, err := ini.LoadSources(
						ini.LoadOptions{AllowPythonMultilineValues: true, SpaceBeforeInlineComment: true},
						[]byte(`
[author]
NAME = U#n#k#n#w#o#n
GITHUB = U;n;k;n;w;o;n
`))
					So(err, ShouldBeNil)
					So(f, ShouldNotBeNil)
					sec := f.Section("author")
					nameValue := sec.Key("NAME").String()
					githubValue := sec.Key("GITHUB").String()
					So(nameValue, ShouldEqual, "U#n#k#n#w#o#n")
					So(githubValue, ShouldEqual, "U;n;k;n;w;o;n")
				})
			})
		})
	})
}
