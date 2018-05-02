// Copyright 2017 Unknwon
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

func TestEmpty(t *testing.T) {
	Convey("Create an empty object", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		// Should only have the default section
		So(len(f.Sections()), ShouldEqual, 1)

		// Default section should not contain any key
		So(len(f.Section("").Keys()), ShouldBeZeroValue)
	})
}

func TestFile_NewSection(t *testing.T) {
	Convey("Create a new section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		sec, err := f.NewSection("author")
		So(err, ShouldBeNil)
		So(sec, ShouldNotBeNil)
		So(sec.Name(), ShouldEqual, "author")

		So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "author"})

		Convey("With duplicated name", func() {
			sec, err := f.NewSection("author")
			So(err, ShouldBeNil)
			So(sec, ShouldNotBeNil)

			// Does nothing if section already exists
			So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "author"})
		})

		Convey("With empty string", func() {
			_, err := f.NewSection("")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFile_NewRawSection(t *testing.T) {
	Convey("Create a new raw section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)
		So(err, ShouldBeNil)
		So(sec, ShouldNotBeNil)
		So(sec.Name(), ShouldEqual, "comments")

		So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "comments"})
		So(f.Section("comments").Body(), ShouldEqual, `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)

		Convey("With duplicated name", func() {
			sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000`)
			So(err, ShouldBeNil)
			So(sec, ShouldNotBeNil)
			So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "comments"})

			// Overwrite previous existed section
			So(f.Section("comments").Body(), ShouldEqual, `1111111111111111111000000000000000001110000`)
		})

		Convey("With empty string", func() {
			_, err := f.NewRawSection("", "")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFile_NewSections(t *testing.T) {
	Convey("Create new sections", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		So(f.NewSections("package", "author"), ShouldBeNil)
		So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "package", "author"})

		Convey("With duplicated name", func() {
			So(f.NewSections("author", "features"), ShouldBeNil)

			// Ignore section already exists
			So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "package", "author", "features"})
		})

		Convey("With empty string", func() {
			So(f.NewSections("", ""), ShouldNotBeNil)
		})
	})
}

func TestFile_GetSection(t *testing.T) {
	Convey("Get a section", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		sec, err := f.GetSection("author")
		So(err, ShouldBeNil)
		So(sec, ShouldNotBeNil)
		So(sec.Name(), ShouldEqual, "author")

		Convey("Section not exists", func() {
			_, err := f.GetSection("404")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestFile_Section(t *testing.T) {
	Convey("Get a section", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		sec := f.Section("author")
		So(sec, ShouldNotBeNil)
		So(sec.Name(), ShouldEqual, "author")

		Convey("Section not exists", func() {
			sec := f.Section("404")
			So(sec, ShouldNotBeNil)
			So(sec.Name(), ShouldEqual, "404")
		})
	})

	Convey("Get default section in lower case with insensitive load", t, func() {
		f, err := ini.InsensitiveLoad([]byte(`
[default]
NAME = ini
VERSION = v1`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		So(f.Section("").Key("name").String(), ShouldEqual, "ini")
		So(f.Section("").Key("version").String(), ShouldEqual, "v1")
	})
}

func TestFile_Sections(t *testing.T) {
	Convey("Get all sections", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		secs := f.Sections()
		names := []string{ini.DEFAULT_SECTION, "author", "package", "package.sub", "features", "types", "array", "note", "comments", "string escapes", "advance"}
		So(len(secs), ShouldEqual, len(names))
		for i, name := range names {
			So(secs[i].Name(), ShouldEqual, name)
		}
	})
}

func TestFile_ChildSections(t *testing.T) {
	Convey("Get child sections by parent name", t, func() {
		f, err := ini.Load([]byte(`
[node]
[node.biz1]
[node.biz2]
[node.biz3]
[node.bizN]
`))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		children := f.ChildSections("node")
		names := []string{"node.biz1", "node.biz2", "node.biz3", "node.bizN"}
		So(len(children), ShouldEqual, len(names))
		for i, name := range names {
			So(children[i].Name(), ShouldEqual, name)
		}
	})
}

func TestFile_SectionStrings(t *testing.T) {
	Convey("Get all section names", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		So(f.SectionStrings(), ShouldResemble, []string{ini.DEFAULT_SECTION, "author", "package", "package.sub", "features", "types", "array", "note", "comments", "string escapes", "advance"})
	})
}

func TestFile_DeleteSection(t *testing.T) {
	Convey("Delete a section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		f.NewSections("author", "package", "features")
		f.DeleteSection("features")
		f.DeleteSection("")
		So(f.SectionStrings(), ShouldResemble, []string{"author", "package"})
	})
}

func TestFile_Append(t *testing.T) {
	Convey("Append a data source", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		So(f.Append(_MINIMAL_CONF, []byte(`
[author]
NAME = Unknwon`)), ShouldBeNil)

		Convey("With bad input", func() {
			So(f.Append(123), ShouldNotBeNil)
			So(f.Append(_MINIMAL_CONF, 123), ShouldNotBeNil)
		})
	})
}

func TestFile_WriteTo(t *testing.T) {
	Convey("Write content to somewhere", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		f.Section("author").Comment = `Information about package author
# Bio can be written in multiple lines.`
		f.Section("author").Key("NAME").Comment = "This is author name"
		f.Section("note").NewBooleanKey("boolean_key")
		f.Section("note").NewKey("more", "notes")

		var buf bytes.Buffer
		_, err = f.WriteTo(&buf)
		So(err, ShouldBeNil)

		golden := "testdata/TestFile_WriteTo.golden"
		if *update {
			ioutil.WriteFile(golden, buf.Bytes(), 0644)
		}

		expected, err := ioutil.ReadFile(golden)
		So(err, ShouldBeNil)
		So(buf.String(), ShouldEqual, string(expected))
	})

	Convey("Support multiline comments", t, func() {
		f := ini.Empty()
		f.Section("").Key("test").Comment = "Multiline\nComment"

		var buf bytes.Buffer
		_, err := f.WriteTo(&buf)
		So(err, ShouldBeNil)

		So(buf.String(), ShouldEqual, `; Multiline
; Comment
test = 

`)

	})
}

func TestFile_SaveTo(t *testing.T) {
	Convey("Write content to somewhere", t, func() {
		f, err := ini.Load(_FULL_CONF)
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		So(f.SaveTo("testdata/conf_out.ini"), ShouldBeNil)
		So(f.SaveToIndent("testdata/conf_out.ini", "\t"), ShouldBeNil)
	})
}
