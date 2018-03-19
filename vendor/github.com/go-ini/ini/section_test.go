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
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"gopkg.in/ini.v1"
)

func TestSection_SetBody(t *testing.T) {
	Convey("Set body of raw section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		sec, err := f.NewRawSection("comments", `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)
		So(err, ShouldBeNil)
		So(sec, ShouldNotBeNil)
		So(sec.Body(), ShouldEqual, `1111111111111111111000000000000000001110000
111111111111111111100000000000111000000000`)

		sec.SetBody("1111111111111111111000000000000000001110000")
		So(sec.Body(), ShouldEqual, `1111111111111111111000000000000000001110000`)

		Convey("Set for non-raw section", func() {
			sec, err := f.NewSection("author")
			So(err, ShouldBeNil)
			So(sec, ShouldNotBeNil)
			So(sec.Body(), ShouldBeEmpty)

			sec.SetBody("1111111111111111111000000000000000001110000")
			So(sec.Body(), ShouldBeEmpty)
		})
	})
}

func TestSection_NewKey(t *testing.T) {
	Convey("Create a new key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		So(k.Name(), ShouldEqual, "NAME")
		So(k.Value(), ShouldEqual, "ini")

		Convey("With duplicated name", func() {
			k, err := f.Section("").NewKey("NAME", "ini.v1")
			So(err, ShouldBeNil)
			So(k, ShouldNotBeNil)

			// Overwrite previous existed key
			So(k.Value(), ShouldEqual, "ini.v1")
		})

		Convey("With empty string", func() {
			_, err := f.Section("").NewKey("", "")
			So(err, ShouldNotBeNil)
		})
	})

	Convey("Create keys with same name and allow shadow", t, func() {
		f, err := ini.ShadowLoad([]byte(""))
		So(err, ShouldBeNil)
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("NAME", "ini.v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(k.ValueWithShadows(), ShouldResemble, []string{"ini", "ini.v1"})
	})
}

func TestSection_NewBooleanKey(t *testing.T) {
	Convey("Create a new boolean key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewBooleanKey("start-ssh-server")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		So(k.Name(), ShouldEqual, "start-ssh-server")
		So(k.Value(), ShouldEqual, "true")

		Convey("With empty string", func() {
			_, err := f.Section("").NewBooleanKey("")
			So(err, ShouldNotBeNil)
		})
	})
}

func TestSection_GetKey(t *testing.T) {
	Convey("Get a key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		k, err = f.Section("").GetKey("NAME")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		So(k.Name(), ShouldEqual, "NAME")
		So(k.Value(), ShouldEqual, "ini")

		Convey("Key not exists", func() {
			_, err := f.Section("").GetKey("404")
			So(err, ShouldNotBeNil)
		})

		Convey("Key exists in parent section", func() {
			k, err := f.Section("parent").NewKey("AGE", "18")
			So(err, ShouldBeNil)
			So(k, ShouldNotBeNil)

			k, err = f.Section("parent.child.son").GetKey("AGE")
			So(err, ShouldBeNil)
			So(k, ShouldNotBeNil)
			So(k.Value(), ShouldEqual, "18")
		})
	})
}

func TestSection_HasKey(t *testing.T) {
	Convey("Check if a key exists", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(f.Section("").HasKey("NAME"), ShouldBeTrue)
		So(f.Section("").Haskey("NAME"), ShouldBeTrue)
		So(f.Section("").HasKey("404"), ShouldBeFalse)
		So(f.Section("").Haskey("404"), ShouldBeFalse)
	})
}

func TestSection_HasValue(t *testing.T) {
	Convey("Check if contains a value in any key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(f.Section("").HasValue("ini"), ShouldBeTrue)
		So(f.Section("").HasValue("404"), ShouldBeFalse)
	})
}

func TestSection_Key(t *testing.T) {
	Convey("Get a key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		k = f.Section("").Key("NAME")
		So(k, ShouldNotBeNil)
		So(k.Name(), ShouldEqual, "NAME")
		So(k.Value(), ShouldEqual, "ini")

		Convey("Key not exists", func() {
			k := f.Section("").Key("404")
			So(k, ShouldNotBeNil)
			So(k.Name(), ShouldEqual, "404")
		})

		Convey("Key exists in parent section", func() {
			k, err := f.Section("parent").NewKey("AGE", "18")
			So(err, ShouldBeNil)
			So(k, ShouldNotBeNil)

			k = f.Section("parent.child.son").Key("AGE")
			So(k, ShouldNotBeNil)
			So(k.Value(), ShouldEqual, "18")
		})
	})
}

func TestSection_Keys(t *testing.T) {
	Convey("Get all keys in a section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("VERSION", "v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		keys := f.Section("").Keys()
		names := []string{"NAME", "VERSION", "IMPORT_PATH"}
		So(len(keys), ShouldEqual, len(names))
		for i, name := range names {
			So(keys[i].Name(), ShouldEqual, name)
		}
	})
}

func TestSection_ParentKeys(t *testing.T) {
	Convey("Get all keys of parent sections", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("package").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("package").NewKey("VERSION", "v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("package").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		keys := f.Section("package.sub.sub2").ParentKeys()
		names := []string{"NAME", "VERSION", "IMPORT_PATH"}
		So(len(keys), ShouldEqual, len(names))
		for i, name := range names {
			So(keys[i].Name(), ShouldEqual, name)
		}
	})
}

func TestSection_KeyStrings(t *testing.T) {
	Convey("Get all key names in a section", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("VERSION", "v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(f.Section("").KeyStrings(), ShouldResemble, []string{"NAME", "VERSION", "IMPORT_PATH"})
	})
}

func TestSection_KeyHash(t *testing.T) {
	Convey("Get clone of key hash", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("VERSION", "v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)
		k, err = f.Section("").NewKey("IMPORT_PATH", "gopkg.in/ini.v1")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		hash := f.Section("").KeysHash()
		relation := map[string]string{
			"NAME":        "ini",
			"VERSION":     "v1",
			"IMPORT_PATH": "gopkg.in/ini.v1",
		}
		for k, v := range hash {
			So(v, ShouldEqual, relation[k])
		}
	})
}

func TestSection_DeleteKey(t *testing.T) {
	Convey("Delete a key", t, func() {
		f := ini.Empty()
		So(f, ShouldNotBeNil)

		k, err := f.Section("").NewKey("NAME", "ini")
		So(err, ShouldBeNil)
		So(k, ShouldNotBeNil)

		So(f.Section("").HasKey("NAME"), ShouldBeTrue)
		f.Section("").DeleteKey("NAME")
		So(f.Section("").HasKey("NAME"), ShouldBeFalse)
	})
}
