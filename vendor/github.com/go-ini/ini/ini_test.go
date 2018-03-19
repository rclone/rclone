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

		// Vaildate values make sure all sources are loaded correctly
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
}

func TestLoadSources(t *testing.T) {
	Convey("Load from data sources with options", t, func() {
		Convey("Ignore nonexistent files", func() {
			f, err := ini.LooseLoad(_NOT_FOUND_CONF, _MINIMAL_CONF)
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			Convey("Inverse case", func() {
				_, err = ini.Load(_NOT_FOUND_CONF)
				So(err, ShouldNotBeNil)
			})
		})

		Convey("Insensitive to section and key names", func() {
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

		Convey("Ignore continuation lines", func() {
			f, err := ini.LoadSources(ini.LoadOptions{
				IgnoreContinuation: true,
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
				f, err := ini.Load([]byte(`
key1=a\b\
key2=c\d\`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("key1").String(), ShouldEqual, `a\bkey2=c\d`)
			})
		})

		Convey("Ignore inline comments", func() {
			f, err := ini.LoadSources(ini.LoadOptions{
				IgnoreInlineComment: true,
			}, []byte(`
key1=value ;comment
key2=value2 #comment2`))
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			So(f.Section("").Key("key1").String(), ShouldEqual, `value ;comment`)
			So(f.Section("").Key("key2").String(), ShouldEqual, `value2 #comment2`)

			Convey("Inverse case", func() {
				f, err := ini.Load([]byte(`
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
				AllowBooleanKeys: true,
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
				_, err := ini.Load([]byte(`
key1=hello
#key2
key3`))
				So(err, ShouldNotBeNil)
			})
		})

		Convey("Allow shadow keys", func() {
			f, err := ini.ShadowLoad([]byte(`
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
				f, err := ini.Load([]byte(`
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
				UnescapeValueDoubleQuotes: true,
			}, []byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			So(f.Section("").Key("create_repo").String(), ShouldEqual, `创建了仓库 <a href="%s">%s</a>`)

			Convey("Inverse case", func() {
				f, err := ini.Load([]byte(`
create_repo="创建了仓库 <a href=\"%s\">%s</a>"`))
				So(err, ShouldBeNil)
				So(f, ShouldNotBeNil)

				So(f.Section("").Key("create_repo").String(), ShouldEqual, `"创建了仓库 <a href=\"%s\">%s</a>"`)
			})
		})

		Convey("Unescape comment symbols inside value", func() {
			f, err := ini.LoadSources(ini.LoadOptions{
				IgnoreInlineComment:         true,
				UnescapeValueCommentSymbols: true,
			}, []byte(`
key = test value <span style="color: %s\; background: %s">more text</span>
`))
			So(err, ShouldBeNil)
			So(f, ShouldNotBeNil)

			So(f.Section("").Key("key").String(), ShouldEqual, `test value <span style="color: %s; background: %s">more text</span>`)
		})

		Convey("Allow unparseable sections", func() {
			f, err := ini.LoadSources(ini.LoadOptions{
				Insensitive:         true,
				UnparseableSections: []string{"core_lesson", "comments"},
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
				_, err := ini.Load([]byte(`
[CORE_LESSON]
my lesson state data – 1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000 – end my lesson state data`))
				So(err, ShouldNotBeNil)
			})
		})
	})
}
